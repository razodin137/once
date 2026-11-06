package command

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/htmltext"
)

const showTimeout = 15 * time.Second

type showCommand struct {
	cmd    *cobra.Command
	client *http.Client
}

func newShowCommand() *showCommand {
	s := &showCommand{
		client: &http.Client{Timeout: showTimeout},
	}
	s.cmd = &cobra.Command{
		Use:   "show <host>",
		Short: "Print an application's page as text",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(s.run),
	}
	return s
}

// Private

func (s *showCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	app, err := findApplication(ns, args[0])
	if err != nil {
		return err
	}

	page, err := s.page(ctx, app.URL())
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), page)

	return nil
}

func (s *showCommand) page(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	req.Header.Set("User-Agent", "once")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("fetching %s: %s", pageURL, resp.Status)
	}

	if contentType := resp.Header.Get("Content-Type"); !isTextual(contentType) {
		return "", fmt.Errorf("fetching %s: unexpected content type %q", pageURL, contentType)
	}

	page, err := htmltext.Render(resp.Body, resp.Request.URL)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", pageURL, err)
	}

	return page, nil
}

// Helpers

func isTextual(contentType string) bool {
	return contentType == "" ||
		strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "html")
}
