package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

func TestParseEnvVars(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		f := &settingsFlags{}
		result, err := f.parseEnvVars()
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("valid pairs", func(t *testing.T) {
		f := &settingsFlags{env: []string{"FOO=bar", "BAZ=qux"}}
		result, err := f.parseEnvVars()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "qux"}, result)
	})

	t.Run("value containing equals", func(t *testing.T) {
		f := &settingsFlags{env: []string{"DSN=postgres://host?opt=val"}}
		result, err := f.parseEnvVars()
		require.NoError(t, err)
		assert.Equal(t, "postgres://host?opt=val", result["DSN"])
	})

	t.Run("missing equals", func(t *testing.T) {
		f := &settingsFlags{env: []string{"INVALID"}}
		_, err := f.parseEnvVars()
		assert.ErrorContains(t, err, "must be in KEY=VALUE format")
	})

	t.Run("empty key", func(t *testing.T) {
		f := &settingsFlags{env: []string{"=value"}}
		_, err := f.parseEnvVars()
		assert.ErrorContains(t, err, "key must not be empty")
	})

	t.Run("empty value is valid", func(t *testing.T) {
		f := &settingsFlags{env: []string{"KEY="}}
		result, err := f.parseEnvVars()
		require.NoError(t, err)
		assert.Equal(t, "", result["KEY"])
	})

	t.Run("duplicate keys last wins", func(t *testing.T) {
		f := &settingsFlags{env: []string{"KEY=first", "KEY=second"}}
		result, err := f.parseEnvVars()
		require.NoError(t, err)
		assert.Equal(t, "second", result["KEY"])
	})
}

func TestDeployTarget(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()

		image, source := deployTarget(dir)
		assert.Equal(t, docker.StaticImage, image)
		assert.Equal(t, dir, source)
	})

	t.Run("image reference", func(t *testing.T) {
		image, source := deployTarget("ghcr.io/basecamp/once-campfire:main")
		assert.Equal(t, "ghcr.io/basecamp/once-campfire:main", image)
		assert.Empty(t, source)
	})

	t.Run("missing path", func(t *testing.T) {
		image, source := deployTarget("/no/such/folder")
		assert.Equal(t, "/no/such/folder", image)
		assert.Empty(t, source)
	})
}

func TestWithSourceHash(t *testing.T) {
	t.Run("image deploy leaves the hash empty", func(t *testing.T) {
		settings, err := withSourceHash(docker.ApplicationSettings{Image: "image:latest"}, "")
		require.NoError(t, err)
		assert.Empty(t, settings.SourceHash)
	})

	t.Run("folder deploy records the folder hash", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644))

		settings, err := withSourceHash(docker.ApplicationSettings{Source: dir}, dir)
		require.NoError(t, err)

		hash, err := docker.HashSource(dir)
		require.NoError(t, err)
		assert.Equal(t, hash, settings.SourceHash)
	})

	t.Run("changed folder hashes differently", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "index.html")
		require.NoError(t, os.WriteFile(file, []byte("hi"), 0o644))

		before, err := withSourceHash(docker.ApplicationSettings{Source: dir}, dir)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(file, []byte("bye"), 0o644))

		after, err := withSourceHash(docker.ApplicationSettings{Source: dir}, dir)
		require.NoError(t, err)
		assert.NotEqual(t, before.SourceHash, after.SourceHash)
	})

	t.Run("missing folder returns an error", func(t *testing.T) {
		_, err := withSourceHash(docker.ApplicationSettings{}, filepath.Join(t.TempDir(), "missing"))
		assert.Error(t, err)
	})
}

func TestSiteToReplace(t *testing.T) {
	ns, err := docker.NewNamespace("once", docker.WithApplications(
		docker.ApplicationSettings{Name: "site.abc123", Image: docker.StaticImage, Source: "/srv/site", Host: "site.localhost"},
		docker.ApplicationSettings{Name: "app.def456", Image: "image:latest", Host: "app.localhost"},
	))
	require.NoError(t, err)

	t.Run("unused host", func(t *testing.T) {
		app, err := siteToReplace(ns, "new.localhost", "/srv/new")
		require.NoError(t, err)
		assert.Nil(t, app)
	})

	t.Run("deploying a folder over a static site", func(t *testing.T) {
		app, err := siteToReplace(ns, "site.localhost", "/srv/site-v2")
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "site.abc123", app.Settings.Name)
	})

	t.Run("deploying a folder over an application", func(t *testing.T) {
		app, err := siteToReplace(ns, "app.localhost", "/srv/app")
		assert.ErrorIs(t, err, docker.ErrHostnameInUse)
		assert.Nil(t, app)
	})

	t.Run("deploying an image over an existing host", func(t *testing.T) {
		app, err := siteToReplace(ns, "site.localhost", "")
		assert.ErrorIs(t, err, docker.ErrHostnameInUse)
		assert.Nil(t, app)
	})
}

func TestRedeploySettings(t *testing.T) {
	existing := docker.ApplicationSettings{
		Name:       "site.abc123",
		Image:      docker.StaticImage,
		Source:     "/srv/site",
		Host:       "site.example.com",
		DisableTLS: false,
		EnvVars:    map[string]string{"KEY": "value"},
		Resources:  docker.ContainerResources{CPUs: 2, MemoryMB: 1024},
		Keys:       docker.Keys{SecretKeyBase: "secret", VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv"},
	}

	t.Run("keeps settings and swaps the source", func(t *testing.T) {
		cmd := &cobra.Command{}
		d := &deployCommand{}
		d.flags.register(cmd)

		result, err := d.redeploySettings(cmd, existing, docker.StaticImage, "/srv/site-v2")
		require.NoError(t, err)

		assert.Equal(t, existing.Name, result.Name)
		assert.Equal(t, existing.Host, result.Host)
		assert.Equal(t, existing.DisableTLS, result.DisableTLS)
		assert.Equal(t, existing.EnvVars, result.EnvVars)
		assert.Equal(t, existing.Resources, result.Resources)
		assert.Equal(t, existing.Keys, result.Keys)
		assert.Equal(t, docker.StaticImage, result.Image)
		assert.Equal(t, "/srv/site-v2", result.Source)
	})

	t.Run("changed flags still apply", func(t *testing.T) {
		cmd := &cobra.Command{}
		d := &deployCommand{}
		d.flags.register(cmd)
		require.NoError(t, cmd.Flags().Set("memory", "2048"))

		result, err := d.redeploySettings(cmd, existing, docker.StaticImage, "/srv/site-v2")
		require.NoError(t, err)

		assert.Equal(t, 2048, result.Resources.MemoryMB)
		assert.Equal(t, existing.Resources.CPUs, result.Resources.CPUs)
		assert.Equal(t, existing.Keys, result.Keys)
	})

	t.Run("invalid flags return an error", func(t *testing.T) {
		cmd := &cobra.Command{}
		d := &deployCommand{}
		d.flags.register(cmd)
		require.NoError(t, cmd.Flags().Set("env", "INVALID"))

		_, err := d.redeploySettings(cmd, existing, docker.StaticImage, "/srv/site-v2")
		assert.ErrorContains(t, err, "must be in KEY=VALUE format")
	})
}

func TestBuildSettingsImageRequired(t *testing.T) {
	f := &settingsFlags{}
	_, err := f.buildSettings(&cobra.Command{}, "", "app.example.com")
	assert.ErrorIs(t, err, docker.ErrImageRequired)
}

func TestBuildSettingsAutoBackupRequiresPath(t *testing.T) {
	t.Run("auto-backup without path", func(t *testing.T) {
		f := &settingsFlags{autoBackup: true}
		_, err := f.buildSettings(&cobra.Command{}, "image:latest", "app.example.com")
		assert.ErrorIs(t, err, docker.ErrAutoBackupWithoutPath)
	})

	t.Run("auto-backup with path", func(t *testing.T) {
		f := &settingsFlags{autoBackup: true, backupPath: "/backups"}
		_, err := f.buildSettings(&cobra.Command{}, "image:latest", "app.example.com")
		require.NoError(t, err)
	})
}

func TestBuildSettingsRegistry(t *testing.T) {
	newCmd := func() (*cobra.Command, *settingsFlags) {
		cmd := &cobra.Command{}
		f := &settingsFlags{}
		f.register(cmd)
		return cmd, f
	}

	t.Run("no registry flags leaves registry empty", func(t *testing.T) {
		cmd, f := newCmd()
		s, err := f.buildSettings(cmd, "image:latest", "app.example.com")
		require.NoError(t, err)
		assert.True(t, s.Registry.Empty())
	})

	t.Run("username and password set the scoped credentials", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-username", "user"))
		require.NoError(t, cmd.Flags().Set("registry-password", "pass"))

		s, err := f.buildSettings(cmd, "image:latest", "app.example.com")
		require.NoError(t, err)
		assert.Equal(t, docker.RegistrySettings{Host: "index.docker.io", Username: "user", Password: "pass"}, s.Registry)
	})

	t.Run("password from stdin trims the trailing newline", func(t *testing.T) {
		cmd, f := newCmd()
		cmd.SetIn(strings.NewReader("hunter2\n"))
		require.NoError(t, cmd.Flags().Set("registry-username", "user"))
		require.NoError(t, cmd.Flags().Set("registry-password-stdin", "true"))

		s, err := f.buildSettings(cmd, "image:latest", "app.example.com")
		require.NoError(t, err)
		assert.Equal(t, "hunter2", s.Registry.Password)
	})

	t.Run("password without username", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-password", "pass"))

		_, err := f.buildSettings(cmd, "image:latest", "app.example.com")
		assert.ErrorIs(t, err, docker.ErrRegistryPasswordWithoutUsername)
	})

	t.Run("username without password", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-username", "user"))

		_, err := f.buildSettings(cmd, "image:latest", "app.example.com")
		assert.ErrorIs(t, err, docker.ErrRegistryUsernameWithoutPassword)
	})

	t.Run("password and password-stdin are mutually exclusive", func(t *testing.T) {
		cmd, _ := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-password", "pass"))
		require.NoError(t, cmd.Flags().Set("registry-password-stdin", "true"))

		assert.Error(t, cmd.ValidateFlagGroups())
	})
}
