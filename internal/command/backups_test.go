package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

func TestBackupsListsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "myapp-20250101-090000.tar.gz"), []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "myapp-20250201-090000.tar.gz"), []byte("newer!"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644))

	ns := listNamespace(t, docker.ApplicationSettings{
		Name:   "myapp",
		Host:   "myapp.localhost",
		Image:  "example/app:latest",
		Backup: docker.BackupSettings{Path: dir},
	})

	now := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	b := newBackupsCommand()
	b.now = func() time.Time { return now }

	var out bytes.Buffer
	b.cmd.SetOut(&out)

	require.NoError(t, b.run(context.Background(), ns, b.cmd, []string{"myapp.localhost"}))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)

	assert.Contains(t, lines[0], "myapp-20250201-090000.tar.gz")
	assert.Contains(t, lines[0], "1 Feb 2025, 09:00")
	assert.Contains(t, lines[0], "27d 15h")
	assert.Contains(t, lines[0], "6 B")

	assert.Contains(t, lines[1], "myapp-20250101-090000.tar.gz")
	assert.Contains(t, lines[1], "58d 15h")
}

func TestBackupsEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	ns := listNamespace(t, docker.ApplicationSettings{
		Name:   "myapp",
		Host:   "myapp.localhost",
		Image:  "example/app:latest",
		Backup: docker.BackupSettings{Path: dir},
	})

	b := newBackupsCommand()

	var out bytes.Buffer
	b.cmd.SetOut(&out)

	require.NoError(t, b.run(context.Background(), ns, b.cmd, []string{"myapp.localhost"}))

	assert.Equal(t, "No backups found in "+dir+".\n", out.String())
}

func TestBackupsNoLocation(t *testing.T) {
	ns := listNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.localhost",
		Image: "example/app:latest",
	})

	b := newBackupsCommand()

	var out bytes.Buffer
	b.cmd.SetOut(&out)

	require.NoError(t, b.run(context.Background(), ns, b.cmd, []string{"myapp.localhost"}))

	assert.Equal(t, "No backups: no backup location is configured for myapp.localhost.\n", out.String())
}

func TestBackupsUnknownHost(t *testing.T) {
	ns := listNamespace(t)

	b := newBackupsCommand()

	err := b.run(context.Background(), ns, b.cmd, []string{"missing.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestFormatBackupSize(t *testing.T) {
	assert.Equal(t, "512 B", formatBackupSize(512))
	assert.Equal(t, "2 KB", formatBackupSize(2_000))
	assert.Equal(t, "1.5 MB", formatBackupSize(1_500_000))
	assert.Equal(t, "2.0 GB", formatBackupSize(2_000_000_000))
}

func TestFormatBackupAge(t *testing.T) {
	assert.Equal(t, "now", formatBackupAge(30*time.Second))
	assert.Equal(t, "5m", formatBackupAge(5*time.Minute))
	assert.Equal(t, "3h", formatBackupAge(3*time.Hour))
	assert.Equal(t, "2d 4h", formatBackupAge(52*time.Hour))
	assert.Equal(t, "3d", formatBackupAge(72*time.Hour))
}
