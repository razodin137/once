package command

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

func TestShowPrintsApplicationPageAsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/", r.URL.Path)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<h1>Hello</h1><p>Visit <a href="/about">about</a></p>`)
	}))
	defer server.Close()

	ns := showNamespace(t, server)
	s := showCommandFor(t)

	var out bytes.Buffer
	s.cmd.SetOut(&out)

	err := s.run(context.Background(), ns, s.cmd, []string{server.Listener.Addr().String()})
	require.NoError(t, err)
	assert.Equal(t, "Hello\n\nVisit about <"+server.URL+"/about>\n", out.String())
}

func TestShowUnknownHost(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	s := showCommandFor(t)

	err = s.run(context.Background(), ns, s.cmd, []string{"missing.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestShowUnreachableApplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	ns := showNamespace(t, server)
	s := showCommandFor(t)

	err := s.run(context.Background(), ns, s.cmd, []string{server.Listener.Addr().String()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching http://"+server.Listener.Addr().String())
}

func TestShowApplicationErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ns := showNamespace(t, server)
	s := showCommandFor(t)

	err := s.run(context.Background(), ns, s.cmd, []string{server.Listener.Addr().String()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

func TestShowRejectsNonTextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
	}))
	defer server.Close()

	ns := showNamespace(t, server)
	s := showCommandFor(t)

	err := s.run(context.Background(), ns, s.cmd, []string{server.Listener.Addr().String()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unexpected content type "image/png"`)
}

// Helpers

func showNamespace(t *testing.T, server *httptest.Server) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{
			Name:       "myapp",
			Host:       server.Listener.Addr().String(),
			DisableTLS: true,
		},
	))
	require.NoError(t, err)

	return ns
}

func showCommandFor(t *testing.T) *showCommand {
	t.Helper()

	s := newShowCommand()
	s.cmd.SetOut(io.Discard)

	return s
}
