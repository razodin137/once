package command

import (
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
