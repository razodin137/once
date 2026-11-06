package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/basecamp/once/internal/docker"
)

func TestContentsCardShowsAppFacts(t *testing.T) {
	app := &docker.Application{
		Running:      true,
		RunningSince: time.Now().Add(-2 * time.Hour),
		DeployedAt:   time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC),
		Settings: docker.ApplicationSettings{
			Name:  "writebook",
			Host:  "books.example.com",
			Image: "ghcr.io/basecamp/writebook:latest",
		},
	}
	backup := &docker.OperationResult{At: time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC)}

	view := ansi.Strip(NewContentsCard(app, backup, 80).View())

	assert.Contains(t, view, "Writebook")
	assert.Contains(t, view, "WR")
	assert.Contains(t, view, "books.example.com")
	assert.Contains(t, view, "IMAGE")
	assert.Contains(t, view, "ghcr.io/basecamp/writebook:latest")
	assert.Contains(t, view, "UPTIME")
	assert.Contains(t, view, "2h")
	assert.Contains(t, view, "LAST DEPLOY")
	assert.Contains(t, view, "2 Nov 2026")
	assert.Contains(t, view, "LAST BACKUP")
	assert.Contains(t, view, "5 Nov 2026")
}

func TestContentsCardFolderApp(t *testing.T) {
	app := &docker.Application{
		Settings: docker.ApplicationSettings{
			Name:   "my-site",
			Host:   "site.example.com",
			Source: "/home/me/site",
		},
	}

	view := ansi.Strip(NewContentsCard(app, nil, 80).View())

	assert.Contains(t, view, "My Site")
	assert.Contains(t, view, "FOLDER")
	assert.Contains(t, view, "/home/me/site")
	assert.Contains(t, view, "stopped")
	assert.Contains(t, view, "Never")
}

func TestContentsCardFailedBackup(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{Name: "app"}}
	backup := &docker.OperationResult{
		At:    time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC),
		Error: "disk full",
	}

	view := ansi.Strip(NewContentsCard(app, backup, 80).View())

	assert.Contains(t, view, "failed")
}

func TestContentsCardFitsWidth(t *testing.T) {
	app := &docker.Application{
		Settings: docker.ApplicationSettings{
			Name:  "very-long-application-name",
			Host:  "a-very-long-hostname.example.com",
			Image: "ghcr.io/example/very-long-image-name:latest",
		},
	}

	view := NewContentsCard(app, nil, 50).View()

	for line := range strings.SplitSeq(ansi.Strip(view), "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), 50)
	}
}

func TestContentsCardCloseKey(t *testing.T) {
	card := NewContentsCard(&docker.Application{}, nil, 80)

	_, cmd := card.Update(keyPressMsg("esc"))

	assert.NotNil(t, cmd)
	assert.IsType(t, ContentsCardCloseMsg{}, cmd())
}
