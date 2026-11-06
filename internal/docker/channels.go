package docker

import "time"

type Channel string

const (
	ChannelRunning   Channel = "running"
	ChannelStopped   Channel = "stopped"
	ChannelDeploying Channel = "deploying"
	ChannelStale     Channel = "stale"
	ChannelUnhealthy Channel = "unhealthy"
	ChannelNoBackup  Channel = "no-backup"
	ChannelIdle      Channel = "idle"
)

const (
	deployingWindow = 30 * time.Second
	idleAfter       = 30 * 24 * time.Hour
)

type ChannelContext struct {
	Now        time.Time
	Stale      bool
	Unhealthy  bool
	LastBackup *OperationResult
}

func (a *Application) Channels(ctx ChannelContext) []Channel {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now()
	}

	deploying := a.deploying(now)

	channels := make([]Channel, 0, 5)
	switch {
	case deploying:
		channels = append(channels, ChannelDeploying)
	case a.Running:
		channels = append(channels, ChannelRunning)
	default:
		channels = append(channels, ChannelStopped)
	}

	if ctx.Stale {
		channels = append(channels, ChannelStale)
	}
	if a.Running && !deploying && ctx.Unhealthy {
		channels = append(channels, ChannelUnhealthy)
	}
	if noBackup(ctx.LastBackup) {
		channels = append(channels, ChannelNoBackup)
	}
	if a.idle(now) {
		channels = append(channels, ChannelIdle)
	}

	return channels
}

// Private

func (a *Application) deploying(now time.Time) bool {
	return !a.DeployedAt.IsZero() && now.Sub(a.DeployedAt) < deployingWindow
}

func (a *Application) idle(now time.Time) bool {
	if !a.Running {
		return false
	}

	since := a.RunningSince
	if since.IsZero() {
		since = a.DeployedAt
	}
	return !since.IsZero() && now.Sub(since) >= idleAfter
}

// Helpers

func noBackup(result *OperationResult) bool {
	return result == nil || result.At.IsZero() || result.Error != ""
}
