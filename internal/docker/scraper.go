package docker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Sample represents CPU and memory stats for a single scrape interval
type Sample struct {
	Timestamp   time.Time
	CPUPercent  float64
	MemoryBytes uint64
}

// ScraperSettings configures the docker stats scraper
type ScraperSettings struct {
	BufferSize int
}

const defaultStatBufferSize = 200

func (s ScraperSettings) withDefaults() ScraperSettings {
	if s.BufferSize == 0 {
		s.BufferSize = defaultStatBufferSize
	}
	return s
}

// statsClient defines the Docker API operations needed by the scraper
type statsClient interface {
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerStats(ctx context.Context, containerID string, options client.ContainerStatsOptions) (client.ContainerStatsResult, error)
}

// Scraper collects Docker container stats using persistent streaming connections.
// Background goroutines maintain streams for each container and update latest stats.
// Scrape() snapshots the current values without blocking on Docker API calls.
type Scraper struct {
	settings ScraperSettings
	client   statsClient
	prefix   string

	mu        sync.RWMutex
	apps      map[string]*appData
	streams   map[string]*streamInfo
	stale     map[string]bool
	lastError error
}

type streamInfo struct {
	containerID string
	cancel      context.CancelFunc
}

type appContainer struct {
	id       string
	settings ApplicationSettings
}

type appData struct {
	samples *RingBuffer[Sample]
	latest  *liveStats
}

type liveStats struct {
	cpuPercent  float64
	memoryBytes uint64
}

func NewScraper(ns *Namespace, settings ScraperSettings) *Scraper {
	settings = settings.withDefaults()
	return &Scraper{
		settings: settings,
		client:   ns.client,
		prefix:   ns.name + "-app-",
		apps:     make(map[string]*appData),
		streams:  make(map[string]*streamInfo),
		stale:    make(map[string]bool),
	}
}

// Fetch returns the last n samples for an app, ordered from newest to oldest.
// If fewer than n samples exist, only the available samples are returned.
func (s *Scraper) Fetch(appName string, n int) []Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.apps[appName]
	if !ok {
		return nil
	}

	return data.samples.FetchNewestFirst(n)
}

func (s *Scraper) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Stale reports whether the folder an app was deployed from has changed since
// that deploy.
func (s *Scraper) Stale(appName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stale[appName]
}

// Scrape snapshots the latest stats from streaming connections and records samples.
// It also ensures streams are running for all current containers.
func (s *Scraper) Scrape(ctx context.Context) {
	containers, err := s.findAppContainers(ctx)
	if err != nil {
		s.setError(err)
		return
	}

	s.updateStreams(ctx, containers)
	s.updateStale(containers)

	now := time.Now()
	s.mu.Lock()
	for appName, data := range s.apps {
		var sample Sample
		if _, running := containers[appName]; running && data.latest != nil {
			sample = Sample{
				Timestamp:   now,
				CPUPercent:  data.latest.cpuPercent,
				MemoryBytes: data.latest.memoryBytes,
			}
		} else {
			sample = Sample{Timestamp: now}
			data.latest = nil
		}

		data.samples.Add(sample)
	}
	s.mu.Unlock()

	s.setError(nil)
}

func (s *Scraper) findAppContainers(ctx context.Context) (map[string]appContainer, error) {
	containers, err := s.client.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]appContainer)
	for _, c := range containers.Items {
		if c.State != "running" {
			continue
		}
		for _, name := range c.Names {
			name = strings.TrimPrefix(name, "/")
			if after, ok := strings.CutPrefix(name, s.prefix); ok {
				if appName, _, ok := cutLast(after, "-"); ok {
					settings, _ := UnmarshalApplicationSettings(c.Labels[labelKey])
					result[appName] = appContainer{id: c.ID, settings: settings}
				}
			}
		}
	}

	return result, nil
}

// Private

func (s *Scraper) updateStale(containers map[string]appContainer) {
	stale := make(map[string]bool)

	for appName, container := range containers {
		settings := container.settings
		if !settings.Static() || settings.SourceHash == "" {
			continue
		}

		hash, err := HashSource(settings.Source)
		if err != nil || hash != settings.SourceHash {
			stale[appName] = true
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stale = stale
}

func (s *Scraper) updateStreams(ctx context.Context, containers map[string]appContainer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Start or restart streams for containers
	for appName, container := range containers {
		if stream, exists := s.streams[appName]; exists {
			if stream.containerID == container.id {
				continue
			}
			// Container ID changed (redeployed), restart stream
			stream.cancel()
		}

		if s.apps[appName] == nil {
			s.apps[appName] = &appData{
				samples: NewRingBuffer[Sample](s.settings.BufferSize),
			}
		}

		streamCtx, cancel := context.WithCancel(ctx)
		s.streams[appName] = &streamInfo{containerID: container.id, cancel: cancel}
		go s.runStream(streamCtx, appName, container.id)
	}

	// Stop streams for removed containers
	for appName, stream := range s.streams {
		if _, exists := containers[appName]; !exists {
			stream.cancel()
			delete(s.streams, appName)
		}
	}
}

func (s *Scraper) runStream(ctx context.Context, appName, containerID string) {
	for {
		s.streamStats(ctx, appName, containerID)

		select {
		case <-ctx.Done():
			return
		case <-time.After(streamRetryDelay):
			// Retry after brief delay if stream disconnected
		}
	}
}

func (s *Scraper) streamStats(ctx context.Context, appName, containerID string) {
	resp, err := s.client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{Stream: true})
	if err != nil {
		return
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var stats container.StatsResponse
		if err := decoder.Decode(&stats); err != nil {
			return
		}

		s.mu.Lock()
		if data := s.apps[appName]; data != nil {
			data.latest = &liveStats{
				cpuPercent:  calculateCPUPercent(&stats),
				memoryBytes: stats.MemoryStats.Usage,
			}
		}
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (s *Scraper) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

// Helpers

func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
	}
	return 0.0
}
