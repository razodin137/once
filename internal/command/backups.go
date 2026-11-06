package command

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

type backupsCommand struct {
	cmd *cobra.Command

	now func() time.Time
}

func newBackupsCommand() *backupsCommand {
	b := &backupsCommand{now: time.Now}
	b.cmd = &cobra.Command{
		Use:   "backups <host>",
		Short: "List an application's backups",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(b.run),
	}
	return b
}

// Private

func (b *backupsCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	app, err := findApplication(ns, args[0])
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	if app.Settings.Backup.Path == "" {
		fmt.Fprintf(w, "No backups: no backup location is configured for %s.\n", app.Settings.Host)
		return nil
	}

	backups, err := app.Backups()
	if err != nil {
		return fmt.Errorf("listing backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Fprintf(w, "No backups found in %s.\n", app.Settings.Backup.Path)
		return nil
	}

	now := b.now()
	for _, backup := range backups {
		fmt.Fprintf(w, "%-19s  %-9s  %8s  %s\n",
			backup.Time.Format("2 Jan 2006, 15:04"),
			formatBackupAge(now.Sub(backup.Time)),
			formatBackupSize(backup.Size),
			backup.Filename)
	}

	return nil
}

// Helpers

func formatBackupAge(age time.Duration) string {
	if age < time.Minute {
		return "now"
	}

	days := int(age.Hours()) / 24
	hours := int(age.Hours()) % 24

	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", int(age.Minutes()))
	}
}

func formatBackupSize(size int64) string {
	const (
		kb = 1_000
		mb = 1_000 * kb
		gb = 1_000 * mb
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.1f GB", float64(size)/gb)
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/mb)
	case size >= kb:
		return fmt.Sprintf("%.0f KB", float64(size)/kb)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
