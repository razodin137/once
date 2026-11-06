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

func TestListPrintsApplications(t *testing.T) {
	ns := listNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.localhost",
		Image: "example/app:latest",
	})
	ns.Applications()[0].Running = true

	l := listCommandFor(t, time.Now(), &docker.State{})

	var out bytes.Buffer
	l.cmd.SetOut(&out)

	require.NoError(t, l.run(context.Background(), ns, l.cmd, nil))
	assert.True(t, strings.Contains(out.String(), "myapp.localhost"))
	assert.True(t, strings.Contains(out.String(), "(running)"))
}

func TestListJSON(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	ns := listNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.localhost",
		Image: "example/app:latest",
	})
	app := ns.Applications()[0]
	app.Running = true
	app.RunningSince = now.Add(-31 * 24 * time.Hour)

	state := &docker.State{Apps: map[string]*docker.AppState{
		"myapp": {LastBackup: docker.OperationResult{At: now.Add(-time.Hour)}},
	}}

	l := listCommandFor(t, now, state)
	l.asJSON = true

	var out bytes.Buffer
	l.cmd.SetOut(&out)

	require.NoError(t, l.run(context.Background(), ns, l.cmd, nil))

	var apps []listApp
	require.NoError(t, json.Unmarshal(out.Bytes(), &apps))
	require.Len(t, apps, 1)

	assert.Equal(t, "myapp.localhost", apps[0].Host)
	assert.Equal(t, "example/app:latest", apps[0].Image)
	assert.Empty(t, apps[0].Folder)
	assert.Equal(t, "running", apps[0].State)
	assert.Equal(t, []docker.Channel{docker.ChannelRunning, docker.ChannelIdle}, apps[0].Channels)
}

func TestListJSONStaticAppUsesFolder(t *testing.T) {
	ns := listNamespace(t, docker.ApplicationSettings{
		Name:   "site",
		Host:   "site.localhost",
		Image:  docker.StaticImage,
		Source: "/srv/site",
	})

	l := listCommandFor(t, time.Now(), &docker.State{})
	l.asJSON = true

	var out bytes.Buffer
	l.cmd.SetOut(&out)

	require.NoError(t, l.run(context.Background(), ns, l.cmd, nil))

	var apps []listApp
	require.NoError(t, json.Unmarshal(out.Bytes(), &apps))
	require.Len(t, apps, 1)

	assert.Equal(t, "site.localhost", apps[0].Host)
	assert.Empty(t, apps[0].Image)
	assert.Equal(t, "/srv/site", apps[0].Folder)
	assert.Equal(t, "stopped", apps[0].State)
	assert.Equal(t, []docker.Channel{docker.ChannelStopped, docker.ChannelNoBackup}, apps[0].Channels)
}

// Helpers

func listNamespace(t *testing.T, apps ...docker.ApplicationSettings) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(apps...))
	require.NoError(t, err)

	return ns
}

func listCommandFor(t *testing.T, now time.Time, state *docker.State) *listCommand {
	t.Helper()

	l := newListCommand()
	l.now = func() time.Time { return now }
	l.loadState = func(context.Context, *docker.Namespace) (*docker.State, error) {
		return state, nil
	}
	return l
}
