package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameFromImageRef(t *testing.T) {
	assert.Equal(t, "once-campfire", NameFromImageRef("ghcr.io/basecamp/once-campfire:main"))
	assert.Equal(t, "once-campfire", NameFromImageRef("ghcr.io/basecamp/once-campfire"))
	assert.Equal(t, "once-campfire", NameFromImageRef("ghcr.io/basecamp/once-campfire:latest"))
}

func TestNameFromPath(t *testing.T) {
	assert.Equal(t, "my-site", NameFromPath("/srv/my-site"))
	assert.Equal(t, "my-site", NameFromPath("/srv/My Site/"))
	assert.Equal(t, "notes-v2", NameFromPath("/srv/notes.v2"))
	assert.Equal(t, "site", NameFromPath("/"))
	assert.Equal(t, "site", NameFromPath("."))

	longName := strings.Repeat("a", 70)
	assert.Equal(t, strings.Repeat("a", 63), NameFromPath("/srv/"+longName))
}

func TestIsLocalhost(t *testing.T) {
	assert.True(t, IsLocalhost("localhost"))
	assert.True(t, IsLocalhost("app.localhost"))
	assert.False(t, IsLocalhost("example.com"))
	assert.False(t, IsLocalhost("localhost.example.com"))
}

func TestCutLast(t *testing.T) {
	before, after, found := cutLast("once-app-myapp-abc123", "-")
	assert.True(t, found)
	assert.Equal(t, "once-app-myapp", before)
	assert.Equal(t, "abc123", after)

	before, after, found = cutLast("nosep", "-")
	assert.False(t, found)
	assert.Equal(t, "nosep", before)
	assert.Equal(t, "", after)
}
