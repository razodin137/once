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

func TestBuildServedSiteRendersMarkdown(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "site")

	markdown := "# Guide\n\nSome **bold** text.\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\n[Other](other.md)\n"
	require.NoError(t, os.Mkdir(filepath.Join(source, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "docs", "guide.md"), []byte(markdown), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "docs", "plain.md"), []byte("Just text."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "docs", "notes.txt"), []byte("hi"), 0o644))

	require.NoError(t, BuildServedSite(source, dest))

	guide, err := os.ReadFile(filepath.Join(dest, "docs", "guide.md"))
	require.NoError(t, err)
	page := string(guide)

	assert.Contains(t, page, "<title>Guide · ONCE</title>")
	assert.Contains(t, page, `<h1 id="guide">Guide</h1>`)
	assert.Contains(t, page, "<strong>bold</strong>")
	assert.Contains(t, page, "<table>")
	assert.Contains(t, page, `href="other.md"`)
	assert.NotContains(t, page, "# Guide")

	plain, err := os.ReadFile(filepath.Join(dest, "docs", "plain.md"))
	require.NoError(t, err)
	assert.Contains(t, string(plain), "<title>plain · ONCE</title>")

	index, err := os.ReadFile(filepath.Join(dest, "docs", "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(index), `<a href="guide.md">guide.md</a>`)
	assert.Contains(t, string(index), `<a href="plain.md">plain.md</a>`)
}

func TestBuildServedSiteMarkdownOnlyRoot(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "site")

	require.NoError(t, os.WriteFile(filepath.Join(source, "b.md"), []byte("# Beta\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "a.md"), []byte("# Alpha\n"), 0o644))

	require.NoError(t, BuildServedSite(source, dest))

	root, err := os.ReadFile(filepath.Join(dest, "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(root), `<h1 id="alpha">Alpha</h1>`)
	assert.NotContains(t, string(root), "<table>")
}

func TestBuildServedSiteMarkdownOnlyFolder(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "site")

	require.NoError(t, os.WriteFile(filepath.Join(source, "index.html"), []byte("home"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(source, "notes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "notes", "second.md"), []byte("# Second\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "notes", "first.md"), []byte("# First\n"), 0o644))

	require.NoError(t, BuildServedSite(source, dest))

	notes, err := os.ReadFile(filepath.Join(dest, "notes", "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(notes), `<h1 id="first">First</h1>`)
}

func TestBuildServedSiteMissingSource(t *testing.T) {
	err := BuildServedSite(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "site"))
	assert.Error(t, err)
}

func TestBuildServedSiteSingleFile(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "site")

	page := filepath.Join(source, "hello.html")
	require.NoError(t, os.WriteFile(page, []byte("<!doctype html><h1>Hi</h1>"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "sibling.txt"), []byte("nope"), 0o644))

	require.NoError(t, BuildServedSite(page, dest))

	index, err := os.ReadFile(filepath.Join(dest, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<!doctype html><h1>Hi</h1>", string(index))
	assert.NoFileExists(t, filepath.Join(dest, "hello.html"))
	assert.NoFileExists(t, filepath.Join(dest, "sibling.txt"))

	info, err := os.Stat(filepath.Join(dest, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestServeSiteSingleFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	page := filepath.Join(t.TempDir(), "hello.html")
	require.NoError(t, os.WriteFile(page, []byte("<h1>Hi</h1>"), 0o644))

	path, err := ServeSite("hello.abc123", page)
	require.NoError(t, err)

	index, err := os.ReadFile(filepath.Join(path, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<h1>Hi</h1>", string(index))
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

func TestRemoveServedSitePrunesCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "index.html"), []byte("hi"), 0o644))

	path, err := ServeSite("site.abc123", source)
	require.NoError(t, err)
	require.DirExists(t, path)

	require.NoError(t, RemoveServedSite("site.abc123"))
	assert.NoDirExists(t, path)
	assert.NoDirExists(t, filepath.Join(cache, "once"))
}

func TestRemoveServedSiteKeepsOtherSites(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "index.html"), []byte("hi"), 0o644))

	first, err := ServeSite("first.abc123", source)
	require.NoError(t, err)
	second, err := ServeSite("second.abc123", source)
	require.NoError(t, err)

	require.NoError(t, RemoveServedSite("first.abc123"))
	assert.NoDirExists(t, filepath.Dir(first))
	assert.DirExists(t, filepath.Dir(second))
	assert.DirExists(t, filepath.Join(cache, "once", "sites"))
}

func TestRemoveServedSiteMissing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	require.NoError(t, RemoveServedSite("missing.abc123"))
}

func TestRemoveServedSiteRequiresName(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	require.Error(t, RemoveServedSite(""))
}
