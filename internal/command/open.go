package command

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/browser"
	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/logging"
	"github.com/basecamp/once/internal/ui"
)

type openCommand struct {
	cmd    *cobra.Command
	open   func(string) error
	openUI func(*docker.Namespace) error
}

func newOpenCommand() *openCommand {
	o := &openCommand{
		open: browser.Open,
		openUI: func(ns *docker.Namespace) error {
			return logging.ToLogFile(func() error {
				return ui.Run(ns, "")
			})
		},
	}
	o.cmd = &cobra.Command{
		Use:   "open [host]",
		Short: "Open an application or the dashboard",
		Args:  cobra.MaximumNArgs(1),
		RunE:  WithNamespace(o.run),
	}
	return o
}

// Private

func (o *openCommand) run(_ context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return o.openUI(ns)
	}

	app, err := findApplication(ns, args[0])
	if err != nil {
		return err
	}

	url := app.URL()
	fmt.Fprintln(cmd.OutOrStdout(), url)

	return o.open(url)
}
