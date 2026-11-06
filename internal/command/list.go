package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

var hostStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue)

type listCommand struct {
	cmd *cobra.Command

	asJSON    bool
	now       func() time.Time
	loadState func(context.Context, *docker.Namespace) (*docker.State, error)
}

type listApp struct {
	Host     string           `json:"host"`
	Image    string           `json:"image,omitempty"`
	Folder   string           `json:"folder,omitempty"`
	State    string           `json:"state"`
	Channels []docker.Channel `json:"channels"`
}

func newListCommand() *listCommand {
	l := &listCommand{
		now: time.Now,
		loadState: func(ctx context.Context, ns *docker.Namespace) (*docker.State, error) {
			return ns.LoadState(ctx)
		},
	}
	l.cmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed applications",
		RunE:    WithNamespace(l.run),
	}
	l.cmd.Flags().BoolVar(&l.asJSON, "json", false, "print the applications as JSON")
	return l
}

// Private

func (l *listCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if l.asJSON {
		return l.writeJSON(ctx, ns, w)
	}

	for _, app := range ns.Applications() {
		host := hostStyle.Hyperlink(app.URL()).Render(app.Settings.Host)
		fmt.Fprintf(w, "%s (%s)\n", host, appState(app))
	}

	return nil
}

func (l *listCommand) writeJSON(ctx context.Context, ns *docker.Namespace, w io.Writer) error {
	state, err := l.loadState(ctx, ns)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if state == nil {
		state = &docker.State{}
	}

	apps := make([]listApp, len(ns.Applications()))
	for i, app := range ns.Applications() {
		apps[i] = l.listApp(app, state)
	}

	return json.NewEncoder(w).Encode(apps)
}

func (l *listCommand) listApp(app *docker.Application, state *docker.State) listApp {
	entry := listApp{
		Host:  app.Settings.Host,
		Image: app.Settings.Image,
		State: appState(app),
		Channels: app.Channels(docker.ChannelContext{
			Now:        l.now(),
			LastBackup: state.AppState(app.Settings.Name).LastBackupResult(),
		}),
	}

	if app.Settings.Source != "" {
		entry.Image = ""
		entry.Folder = app.Settings.Source
	}

	return entry
}

// Helpers

func appState(app *docker.Application) string {
	if app.Running {
		return "running"
	}
	return "stopped"
}
