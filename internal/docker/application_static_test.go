package docker

import (
	"strings"
	"testing"

	"github.com/moby/moby/api/types/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVolumeMountsStaticSource(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{
		Source:     "/srv/my-site",
		ServedPath: "/cache/once/sites/site.abc123/0123456789abcdef",
	}}

	mounts := app.volumeMounts(&ApplicationVolume{name: "once-app-my-site"})

	assert.Len(t, mounts, len(AppVolumeMountTargets)+1)
	assert.Contains(t, mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   "/cache/once/sites/site.abc123/0123456789abcdef",
		Target:   staticContentTarget,
		ReadOnly: true,
	})
}

func TestVolumeMountsStaticSourceWithoutServedCopy(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{Source: "/srv/my-site"}}

	mounts := app.volumeMounts(&ApplicationVolume{name: "once-app-my-site"})

	assert.Contains(t, mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   "/srv/my-site",
		Target:   staticContentTarget,
		ReadOnly: true,
	})
}

func TestContainerConfigStatic(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{Source: "/srv/my-site"}}

	config := app.containerConfig(nil)

	require.NotEmpty(t, config.Cmd)
	configText := strings.Join(config.Cmd, " ")
	assert.Contains(t, configText, "location = /up")
	assert.Contains(t, configText, `location ~* \.md$`)
	assert.Contains(t, configText, "default_type text/html")
}

func TestContainerConfigImage(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{Image: "image:latest"}}

	config := app.containerConfig(nil)

	assert.Empty(t, config.Cmd)
}
