package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplicationChannels(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	backup := &OperationResult{At: now.Add(-time.Hour)}

	t.Run("running", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-2*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning}, channels)
	})

	t.Run("stopped", func(t *testing.T) {
		app := channelTestApp(false, now.Add(-2*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelStopped}, channels)
	})

	t.Run("deploying", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-5*time.Second))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelDeploying}, channels)
	})

	t.Run("stale", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-2*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, Stale: true, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning, ChannelStale}, channels)
	})

	t.Run("unhealthy", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-2*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, Unhealthy: true, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning, ChannelUnhealthy}, channels)
	})

	t.Run("deploying is not unhealthy", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-5*time.Second))

		channels := app.Channels(ChannelContext{Now: now, Unhealthy: true, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelDeploying}, channels)
	})

	t.Run("no backup", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-2*time.Hour))

		channels := app.Channels(ChannelContext{Now: now})

		assert.Equal(t, []Channel{ChannelRunning, ChannelNoBackup}, channels)
	})

	t.Run("failed backup counts as no backup", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-2*time.Hour))
		failed := &OperationResult{At: now.Add(-time.Hour), Error: "disk full"}

		channels := app.Channels(ChannelContext{Now: now, LastBackup: failed})

		assert.Equal(t, []Channel{ChannelRunning, ChannelNoBackup}, channels)
	})

	t.Run("idle for thirty days", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-31*24*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning, ChannelIdle}, channels)
	})

	t.Run("not idle before thirty days", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-29*24*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning}, channels)
	})

	t.Run("recent restart is not idle", func(t *testing.T) {
		app := channelTestApp(true, now.Add(-31*24*time.Hour))
		app.RunningSince = now.Add(-24 * time.Hour)

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelRunning}, channels)
	})

	t.Run("stopped app is not idle", func(t *testing.T) {
		app := channelTestApp(false, now.Add(-31*24*time.Hour))

		channels := app.Channels(ChannelContext{Now: now, LastBackup: backup})

		assert.Equal(t, []Channel{ChannelStopped}, channels)
	})
}

// Helpers

func channelTestApp(running bool, deployedAt time.Time) *Application {
	return &Application{
		Running:    running,
		DeployedAt: deployedAt,
		Settings: ApplicationSettings{
			Name: "app",
			Host: "app.example.com",
		},
	}
}
