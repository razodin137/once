package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/metrics"
	"github.com/basecamp/once/internal/mouse"
)

func TestDashboardPanelViewRunningApp(t *testing.T) {
	panel := testPanel(true)
	view := panel.View(false, false, "", true, 120, DashboardScales{})
	stripped := ansi.Strip(view)

	assert.Contains(t, stripped, "app.example.com")
	assert.Contains(t, stripped, "[running]")
}

func TestDashboardPanelViewStoppedApp(t *testing.T) {
	panel := testPanel(false)
	view := panel.View(false, false, "", true, 120, DashboardScales{})
	stripped := ansi.Strip(view)

	assert.Contains(t, stripped, "app.example.com")
	assert.Contains(t, stripped, "[stopped]")
}

func TestDashboardPanelViewShowsNoBackupChip(t *testing.T) {
	state := &docker.State{Apps: map[string]*docker.AppState{"test-app": {}}}
	panel := testPanelWithState(true, state)
	view := panel.View(false, false, "", true, 120, DashboardScales{})

	assert.Contains(t, ansi.Strip(view), "[no backup]")
}

func TestDashboardPanelViewHidesBackupChipForASuccessfulBackup(t *testing.T) {
	state := &docker.State{Apps: map[string]*docker.AppState{
		"test-app": {LastBackup: docker.OperationResult{At: time.Now()}},
	}}
	panel := testPanelWithState(true, state)
	view := panel.View(false, false, "", true, 120, DashboardScales{})

	assert.NotContains(t, ansi.Strip(view), "[no backup]")
}

func TestDashboardPanelViewShowsIdleChip(t *testing.T) {
	state := &docker.State{Apps: map[string]*docker.AppState{
		"test-app": {LastBackup: docker.OperationResult{At: time.Now()}},
	}}
	panel := testPanelWithState(true, state)
	panel.app.DeployedAt = time.Now().Add(-31 * 24 * time.Hour)
	view := panel.View(false, false, "", true, 120, DashboardScales{})

	assert.Contains(t, ansi.Strip(view), "[idle 30d]")
}

func TestDashboardPanelViewShowsDeployingChip(t *testing.T) {
	panel := testPanel(true)
	panel.app.DeployedAt = time.Now()
	view := panel.View(false, false, "", true, 120, DashboardScales{})

	assert.Contains(t, ansi.Strip(view), "[deploying]")
}

func TestDashboardPanelHeightRunning(t *testing.T) {
	panel := testPanel(true)

	fullHeight := panel.Height(true)
	compactHeight := panel.Height(false)

	assert.Equal(t, PanelHeight+2, fullHeight)
	assert.Equal(t, StoppedPanelHeight+2, compactHeight)
}

func TestDashboardPanelHeightStopped(t *testing.T) {
	panel := testPanel(false)

	// Stopped always uses compact height regardless of showDetails
	assert.Equal(t, StoppedPanelHeight+2, panel.Height(true))
	assert.Equal(t, StoppedPanelHeight+2, panel.Height(false))
}

func TestDashboardPanelSelectedHasIndicator(t *testing.T) {
	panel := testPanel(true)
	view := panel.View(true, false, "", true, 80, DashboardScales{})

	// Selected panels have the indicator character
	assert.Contains(t, view, "▐")
	assert.Contains(t, view, "▗")
	assert.Contains(t, view, "▝")
}

func TestDashboardPanelNotSelectedNoIndicator(t *testing.T) {
	panel := testPanel(true)
	view := panel.View(false, false, "", true, 80, DashboardScales{})

	assert.NotContains(t, view, "▐")
	assert.NotContains(t, view, "▗")
	assert.NotContains(t, view, "▝")
}

func TestDashboardPanelNarrowWidthHidesCards(t *testing.T) {
	panel := testPanel(true)
	// Width of 30 is too narrow for cards (need minCardWidth*4+3 = 35 inner)
	view := panel.View(false, false, "", true, 30, DashboardScales{})

	// Should not contain card borders
	assert.NotContains(t, view, "╭─")
}

func TestDashboardPanelWideWidthShowsCards(t *testing.T) {
	panel := testPanel(true)
	view := panel.View(false, false, "", true, 120, DashboardScales{})

	// Cards should have borders
	assert.Contains(t, view, "╭─")
	assert.Contains(t, view, "╰")
}

func TestDashboardPanelTogglingShowsStatus(t *testing.T) {
	panel := testPanel(true)
	view := panel.View(false, true, "", true, 120, DashboardScales{})
	stripped := ansi.Strip(view)

	assert.Contains(t, stripped, "stopping...")
}

func TestDashboardPanelViewHasThreeLines(t *testing.T) {
	panel := testPanel(false)
	view := panel.View(false, false, "", false, 80, DashboardScales{})
	lines := strings.Split(view, "\n")

	// top transition + body lines + bottom transition
	assert.GreaterOrEqual(t, len(lines), 3)
}

func TestRenderChannelChips(t *testing.T) {
	running := renderChannelChips([]channelChip{newChannelChip(docker.ChannelRunning)}, "")
	stopped := renderChannelChips([]channelChip{newChannelChip(docker.ChannelStopped)}, "")
	noBackup := renderChannelChips([]channelChip{newChannelChip(docker.ChannelNoBackup)}, "")

	assert.Contains(t, ansi.Strip(running), "[running]")
	assert.Contains(t, ansi.Strip(stopped), "[stopped]")
	assert.Contains(t, ansi.Strip(noBackup), "[no backup]")
}

func TestRenderChannelChipsMouseTargets(t *testing.T) {
	row := "row " + renderChannelChips([]channelChip{
		newChannelChip(docker.ChannelRunning),
		newChannelChip(docker.ChannelStopped),
	}, "")

	assert.Contains(t, ansi.Strip(mouse.Sweep(row)), "[running] [stopped]")
	assert.Equal(t, channelTarget(docker.ChannelRunning), mouse.Resolve(5, 0))
	assert.Equal(t, channelTarget(docker.ChannelStopped), mouse.Resolve(15, 0))
}

func TestRenderBar(t *testing.T) {
	bar := renderBar(50, 0, 100, Colors.Success, 10)
	assert.NotEmpty(t, bar)
	// Bar contains braille characters
	assert.Contains(t, bar, "⢾")
	assert.Contains(t, bar, "⡷")
}

func TestRenderBarZeroWidth(t *testing.T) {
	assert.Empty(t, renderBar(50, 0, 100, Colors.Success, 0))
}

func TestDashboardPanelShowsFavoriteStar(t *testing.T) {
	state := &docker.State{}
	state.SetFavorite("test-app", true)
	panel := testPanelWithState(true, state)

	assert.Contains(t, ansi.Strip(panel.View(false, false, "", true, 120, DashboardScales{})), favoriteStar)
}

func TestDashboardPanelShowsUnfavoriteStar(t *testing.T) {
	panel := testPanel(true)

	assert.Contains(t, ansi.Strip(panel.View(false, false, "", true, 120, DashboardScales{})), unfavoriteStar)
}

// Helpers

func testPanel(running bool) DashboardPanel {
	return testPanelWithState(running, nil)
}

func testPanelWithState(running bool, state *docker.State) DashboardPanel {
	app := &docker.Application{
		Running: running,
		Settings: docker.ApplicationSettings{
			Name:  "test-app",
			Host:  "app.example.com",
			Image: "ghcr.io/basecamp/test-app:latest",
		},
	}
	scraper := metrics.NewMetricsScraper(metrics.ScraperSettings{})
	dockerScraper := &docker.Scraper{}

	return NewDashboardPanel(app, scraper, dockerScraper, nil, state)
}
