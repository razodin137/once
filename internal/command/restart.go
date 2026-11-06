package command

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

type restartCommand struct {
	cmd *cobra.Command
}

func newRestartCommand() *restartCommand {
	r := &restartCommand{}
	r.cmd = &cobra.Command{
		Use:   "restart <host>",
		Short: "Restart an application",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(r.run),
	}
	return r
}

// Private

func (r *restartCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	host := args[0]

	err := withApplication(ns, host, "restarting", func(app *docker.Application) error {
		return app.Restart(ctx)
	})
	if err != nil {
		return err
	}

	fmt.Printf("Restarted %s\n", host)
	return nil
}
