package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

func TestOpenPrintsAndOpensApplicationURL(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
	))
	require.NoError(t, err)

	var opened string
	o := newOpenCommand()
	o.open = func(url string) error {
		opened = url
		return nil
	}

	var out bytes.Buffer
	o.cmd.SetOut(&out)

	err = o.run(context.Background(), ns, o.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)
	assert.Equal(t, "http://myapp.localhost\n", out.String())
	assert.Equal(t, "http://myapp.localhost", opened)
}

func TestOpenWithoutHostOpensTheDashboard(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	var launched *docker.Namespace
	o := newOpenCommand()
	o.openUI = func(got *docker.Namespace) error {
		launched = got
		return nil
	}

	err = o.run(context.Background(), ns, o.cmd, nil)
	require.NoError(t, err)
	assert.Same(t, ns, launched)
}

func TestOpenUnknownHost(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	o := newOpenCommand()
	o.open = func(string) error {
		t.Fatal("should not open the browser")
		return nil
	}

	err = o.run(context.Background(), ns, o.cmd, []string{"missing.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestOpenReturnsBrowserError(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
	))
	require.NoError(t, err)

	o := newOpenCommand()
	o.open = func(string) error { return assert.AnError }

	err = o.run(context.Background(), ns, o.cmd, []string{"myapp.localhost"})
	require.ErrorIs(t, err, assert.AnError)
}
