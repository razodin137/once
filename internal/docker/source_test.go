package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashSource(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Hi</h1>"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.css"), []byte("body {}"), 0o644))

	hash, err := HashSource(dir)
	require.NoError(t, err)
	assert.Len(t, hash, 64)

	again, err := HashSource(dir)
	require.NoError(t, err)
	assert.Equal(t, hash, again)
}

func TestHashSourceChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "index.html")
	require.NoError(t, os.WriteFile(file, []byte("<h1>Hi</h1>"), 0o644))

	before, err := HashSource(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(file, []byte("<h1>Bye</h1>"), 0o644))

	after, err := HashSource(dir)
	require.NoError(t, err)
	assert.NotEqual(t, before, after)
}

func TestHashSourceChangesWithFileSet(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644))

	before, err := HashSource(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("hi"), 0o644))

	after, err := HashSource(dir)
	require.NoError(t, err)
	assert.NotEqual(t, before, after)
}

func TestHashSourceChangesWithFileName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.html"), []byte("hi"), 0o644))

	before, err := HashSource(dir)
	require.NoError(t, err)

	require.NoError(t, os.Rename(filepath.Join(dir, "one.html"), filepath.Join(dir, "two.html")))

	after, err := HashSource(dir)
	require.NoError(t, err)
	assert.NotEqual(t, before, after)
}

func TestHashSourceSingleFile(t *testing.T) {
	page := filepath.Join(t.TempDir(), "hello.html")
	require.NoError(t, os.WriteFile(page, []byte("<h1>Hi</h1>"), 0o644))

	hash, err := HashSource(page)
	require.NoError(t, err)
	assert.Len(t, hash, 64)

	again, err := HashSource(page)
	require.NoError(t, err)
	assert.Equal(t, hash, again)

	require.NoError(t, os.WriteFile(page, []byte("<h1>Bye</h1>"), 0o644))

	changed, err := HashSource(page)
	require.NoError(t, err)
	assert.NotEqual(t, hash, changed)
}

func TestHashSourceMissingDirectory(t *testing.T) {
	_, err := HashSource(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}
