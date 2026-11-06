package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

const logsPollInterval = 100 * time.Millisecond

type logStream interface {
	Start(ctx context.Context, containerName string)
	Stop()
	FetchSince(version uint64) ([]docker.LogLine, uint64)
}

type logsCommand struct {
	cmd *cobra.Command

	tail int
	grep string

	newStreamer   func(*docker.Namespace, docker.LogStreamerSettings) logStream
	containerName func(context.Context, *docker.Application) (string, error)
}

func newLogsCommand() *logsCommand {
	l := &logsCommand{
		tail: 100,
		newStreamer: func(ns *docker.Namespace, settings docker.LogStreamerSettings) logStream {
			return docker.NewLogStreamer(ns, settings)
		},
		containerName: func(ctx context.Context, app *docker.Application) (string, error) {
			return app.ContainerName(ctx)
		},
	}
	l.cmd = &cobra.Command{
		Use:   "logs <host>",
		Short: "Stream an application's logs",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(l.run),
	}
	l.cmd.Flags().IntVar(&l.tail, "tail", l.tail, "number of historical lines to show")
	l.cmd.Flags().StringVar(&l.grep, "grep", "", "only show lines matching this regular expression")
	return l
}

// Private

func (l *logsCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	if l.tail < 1 {
		return fmt.Errorf("tail must be at least 1")
	}

	pattern, err := grepPattern(l.grep)
	if err != nil {
		return err
	}

	app, err := findApplication(ns, args[0])
	if err != nil {
		return err
	}

	container, err := l.containerName(ctx, app)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	streamer := l.newStreamer(ns, docker.LogStreamerSettings{Tail: l.tail})
	streamer.Start(ctx, container)
	defer streamer.Stop()

	return l.stream(ctx, cmd, streamer, pattern)
}

func (l *logsCommand) stream(ctx context.Context, cmd *cobra.Command, streamer logStream, pattern *regexp.Regexp) error {
	var printed uint64

	ticker := time.NewTicker(logsPollInterval)
	defer ticker.Stop()

	for {
		printed = printLogLines(cmd.OutOrStdout(), cmd.ErrOrStderr(), streamer, printed, pattern)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Helpers

func grepPattern(expression string) (*regexp.Regexp, error) {
	if expression == "" {
		return nil, nil
	}

	pattern, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid grep pattern: %w", err)
	}

	return pattern, nil
}

func printLogLines(stdout, stderr io.Writer, streamer logStream, printed uint64, pattern *regexp.Regexp) uint64 {
	lines, version := streamer.FetchSince(printed)
	for _, line := range lines {
		if pattern != nil && !pattern.MatchString(line.Content) {
			continue
		}

		w := stdout
		if line.IsStderr {
			w = stderr
		}
		fmt.Fprintln(w, line.Content)
	}

	return version
}
