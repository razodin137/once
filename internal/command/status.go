package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

type statusCommand struct {
	cmd *cobra.Command

	asJSON    bool
	now       func() time.Time
	loadState func(context.Context, *docker.Namespace) (*docker.State, error)
}

type statusReport struct {
	Host          string           `json:"host"`
	URL           string           `json:"url"`
	Image         string           `json:"image"`
	Source        string           `json:"source,omitempty"`
	State         string           `json:"state"`
	UptimeSeconds *int64           `json:"uptimeSeconds"`
	RunningSince  *time.Time       `json:"runningSince,omitempty"`
	Resources     statusResources  `json:"resources"`
	LastUpdate    *statusOperation `json:"lastUpdate"`
	LastBackup    *statusOperation `json:"lastBackup"`
}

type statusResources struct {
	CPUs     int `json:"cpus"`
	MemoryMB int `json:"memoryMB"`
}

type statusOperation struct {
	At    time.Time `json:"at"`
	Error string    `json:"error,omitempty"`
}

func newStatusCommand() *statusCommand {
	s := &statusCommand{
		now: time.Now,
		loadState: func(ctx context.Context, ns *docker.Namespace) (*docker.State, error) {
			return ns.LoadState(ctx)
		},
	}
	s.cmd = &cobra.Command{
		Use:   "status <host>",
		Short: "Show an application's status",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(s.run),
	}
	s.cmd.Flags().BoolVar(&s.asJSON, "json", false, "print the status as JSON")
	return s
}

// Private

func (s *statusCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	app, err := findApplication(ns, args[0])
	if err != nil {
		return err
	}

	state, err := s.loadState(ctx, ns)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	report := s.report(app, state)
	if s.asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}

	s.print(cmd.OutOrStdout(), report)
	return nil
}

func (s *statusCommand) report(app *docker.Application, state *docker.State) statusReport {
	report := statusReport{
		Host:       app.Settings.Host,
		URL:        app.URL(),
		Image:      app.Settings.Image,
		Source:     app.Settings.Source,
		State:      "stopped",
		Resources:  statusResources{CPUs: app.Settings.Resources.CPUs, MemoryMB: app.Settings.Resources.MemoryMB},
		LastUpdate: statusOperationFrom(state.AppState(app.Settings.Name).LastUpdateResult()),
		LastBackup: statusOperationFrom(state.AppState(app.Settings.Name).LastBackupResult()),
	}

	if app.Running {
		report.State = "running"
		if !app.RunningSince.IsZero() {
			since := app.RunningSince
			uptime := max(int64(s.now().Sub(since).Seconds()), 0)
			report.RunningSince = &since
			report.UptimeSeconds = &uptime
		}
	}

	return report
}

func (s *statusCommand) print(w io.Writer, report statusReport) {
	writeStatusLine(w, "Host:", report.Host)
	writeStatusLine(w, "URL:", report.URL)
	writeStatusLine(w, "Image:", report.Image)
	if report.Source != "" {
		writeStatusLine(w, "Source:", report.Source)
	}
	writeStatusLine(w, "State:", report.State)
	writeStatusLine(w, "Uptime:", statusUptime(report))
	writeStatusLine(w, "CPUs:", statusLimit(report.Resources.CPUs))
	writeStatusLine(w, "Memory:", statusMemory(report.Resources.MemoryMB))
	writeStatusLine(w, "Last update:", statusOperationText(report.LastUpdate))
	writeStatusLine(w, "Last backup:", statusOperationText(report.LastBackup))
}

// Helpers

func statusOperationFrom(result *docker.OperationResult) *statusOperation {
	if result == nil {
		return nil
	}
	return &statusOperation{At: result.At, Error: result.Error}
}

func writeStatusLine(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%-13s%s\n", label, value)
}

func statusUptime(report statusReport) string {
	if report.UptimeSeconds == nil {
		return "—"
	}
	return formatStatusDuration(time.Duration(*report.UptimeSeconds) * time.Second)
}

func statusLimit(value int) string {
	if value == 0 {
		return "unlimited"
	}
	return fmt.Sprint(value)
}

func statusMemory(memoryMB int) string {
	if memoryMB == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d MB", memoryMB)
}

func statusOperationText(operation *statusOperation) string {
	if operation == nil {
		return "Never"
	}

	text := operation.At.Format("2 Jan 2006, 15:04")
	if operation.Error != "" {
		text += " (failed: " + operation.Error + ")"
	}
	return text
}

func formatStatusDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, hours)
}
