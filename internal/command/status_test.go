package command

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

func TestStatusPrintsApplicationStatus(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	ns := statusNamespace(t)
	app := ns.ApplicationByHost("myapp.localhost")
	app.Running = true
	app.RunningSince = now.Add(-90 * time.Minute)

	state := &docker.State{Apps: map[string]*docker.AppState{
		"myapp": {
			LastUpdate: docker.OperationResult{At: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)},
			LastBackup: docker.OperationResult{
				At:    time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC),
				Error: "disk full",
			},
		},
	}}

	s := statusCommandFor(t, now, state)

	var out bytes.Buffer
	s.cmd.SetOut(&out)

	err := s.run(context.Background(), ns, s.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	expected := strings.Join([]string{
		"Host:        myapp.localhost",
		"URL:         http://myapp.localhost",
		"Image:       example/app:latest",
		"Source:      /srv/site",
		"State:       running",
		"Uptime:      1h 30m",
		"CPUs:        2",
		"Memory:      1024 MB",
		"Last update: 2 Nov 2026, 10:00",
		"Last backup: 5 Nov 2026, 09:00 (failed: disk full)",
	}, "\n") + "\n"
	assert.Equal(t, expected, out.String())
}

func TestStatusPrintsStoppedApplicationWithoutHistory(t *testing.T) {
	ns := statusStoppedNamespace(t)
	s := statusCommandFor(t, time.Now(), &docker.State{})

	var out bytes.Buffer
	s.cmd.SetOut(&out)

	err := s.run(context.Background(), ns, s.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	expected := strings.Join([]string{
		"Host:        myapp.localhost",
		"URL:         http://myapp.localhost",
		"Image:       example/app:latest",
		"State:       stopped",
		"Uptime:      —",
		"CPUs:        unlimited",
		"Memory:      unlimited",
		"Last update: Never",
		"Last backup: Never",
	}, "\n") + "\n"
	assert.Equal(t, expected, out.String())
}

func TestStatusJSON(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	ns := statusNamespace(t)
	app := ns.ApplicationByHost("myapp.localhost")
	app.Running = true
	app.RunningSince = now.Add(-90 * time.Minute)

	state := &docker.State{Apps: map[string]*docker.AppState{
		"myapp": {
			LastUpdate: docker.OperationResult{At: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)},
		},
	}}

	s := statusCommandFor(t, now, state)
	s.asJSON = true

	var out bytes.Buffer
	s.cmd.SetOut(&out)

	err := s.run(context.Background(), ns, s.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	var report statusReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))

	assert.Equal(t, "myapp.localhost", report.Host)
	assert.Equal(t, "http://myapp.localhost", report.URL)
	assert.Equal(t, "example/app:latest", report.Image)
	assert.Equal(t, "/srv/site", report.Source)
	assert.Equal(t, "running", report.State)
	require.NotNil(t, report.UptimeSeconds)
	assert.Equal(t, int64(5400), *report.UptimeSeconds)
	require.NotNil(t, report.RunningSince)
	assert.Equal(t, app.RunningSince, *report.RunningSince)
	assert.Equal(t, statusResources{CPUs: 2, MemoryMB: 1024}, report.Resources)
	require.NotNil(t, report.LastUpdate)
	assert.Equal(t, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC), report.LastUpdate.At)
	assert.Nil(t, report.LastBackup)
}

func TestStatusJSONStoppedHasNoUptime(t *testing.T) {
	ns := statusStoppedNamespace(t)
	s := statusCommandFor(t, time.Now(), &docker.State{})
	s.asJSON = true

	var out bytes.Buffer
	s.cmd.SetOut(&out)

	err := s.run(context.Background(), ns, s.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	var report statusReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))

	assert.Equal(t, "stopped", report.State)
	assert.Nil(t, report.UptimeSeconds)
	assert.Nil(t, report.RunningSince)
	assert.Nil(t, report.LastUpdate)
	assert.Nil(t, report.LastBackup)
}

// Helpers

func statusNamespace(t *testing.T) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(docker.ApplicationSettings{
		Name:       "myapp",
		Host:       "myapp.localhost",
		Image:      "example/app:latest",
		Source:     "/srv/site",
		DisableTLS: true,
		Resources:  docker.ContainerResources{CPUs: 2, MemoryMB: 1024},
	}))
	require.NoError(t, err)

	return ns
}

func statusStoppedNamespace(t *testing.T) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(docker.ApplicationSettings{
		Name:       "myapp",
		Host:       "myapp.localhost",
		Image:      "example/app:latest",
		DisableTLS: true,
	}))
	require.NoError(t, err)

	return ns
}

func statusCommandFor(t *testing.T, now time.Time, state *docker.State) *statusCommand {
	t.Helper()

	s := newStatusCommand()
	s.now = func() time.Time { return now }
	s.loadState = func(context.Context, *docker.Namespace) (*docker.State, error) {
		return state, nil
	}
	return s
}
