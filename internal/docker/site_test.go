package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildServedSite(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "site")

	modified := time.Date(2024, 5, 6, 7, 8, 0, 0, time.Local)
	logo := filepath.Join(source, "logo.png")
	require.NoError(t, os.WriteFile(logo, []byte("12345"), 0o600))
	require.NoError(t, os.Chtimes(logo, modified, modified))
	require.NoError(t, os.WriteFile(filepath.Join(source, "a & b.txt"), []byte("hi"), 0o644))

	require.NoError(t, os.Mkdir(filepath.Join(source, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "docs", "index.html"), []byte("docs home"), 0o644))

	require.NoError(t, os.Mkdir(filepath.Join(source, "photos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "photos", "cat.jpg"), []byte("jpeg"), 0o644))

	require.NoError(t, os.Mkdir(filepath.Join(source, "blank"), 0o755))

	require.NoError(t, BuildServedSite(source, dest))

	root, err := os.ReadFile(filepath.Join(dest, "index.html"))
	require.NoError(t, err)
	page := string(root)

	assert.Contains(t, page, "<h1>"+filepath.Base(source)+"</h1>")
	assert.Contains(t, page, "a &amp; b.txt")
	assert.Contains(t, page, "logo.png")
	assert.Contains(t, page, "5 B")
	assert.Contains(t, page, "2 B")
	assert.Contains(t, page, modified.Format("2006-01-02 15:04"))
	assert.Contains(t, page, `<a href="docs/">docs</a>`)
	assert.Contains(t, page, `<a href="photos/">photos</a>`)
	assert.Contains(t, page, "5 entries")
	assert.NotContains(t, page, `href="../"`)

	assert.Less(t, strings.Index(page, ">docs<"), strings.Index(page, ">photos<"))
	assert.Less(t, strings.Index(page, ">photos<"), strings.Index(page, ">a &amp; b.txt<"))

	docs, err := os.ReadFile(filepath.Join(dest, "docs", "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "docs home", string(docs))

	photos, err := os.ReadFile(filepath.Join(dest, "photos", "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(photos), "cat.jpg")
	assert.Contains(t, string(photos), `href="../"`)
	assert.Contains(t, string(photos), "1 entry")

	blank, err := os.ReadFile(filepath.Join(dest, "blank", "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(blank), "This folder is empty.")

	info, err := os.Stat(filepath.Join(dest, "logo.png"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestBuildServedSiteMissingSource(t *testing.T) {
	err := BuildServedSite(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "site"))
	assert.Error(t, err)
}

func TestServeSite(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "note.txt"), []byte("hi"), 0o644))

	path, err := ServeSite("site.abc123", source)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(path, "index.html"))

	next, err := ServeSite("site.abc123", source)
	require.NoError(t, err)
	assert.NotEqual(t, path, next)

	require.NoError(t, RemoveServedCopy(path))
	assert.NoDirExists(t, path)
	assert.DirExists(t, next)

	require.NoError(t, RemoveServedSite("site.abc123"))
	assert.NoDirExists(t, filepath.Dir(next))
}
