package command

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/system"
)

func TestDoctorPrintsPassingChecks(t *testing.T) {
	ns := doctorNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.example.com",
		Image: "example/app:latest",
	})
	d := passingDoctor(t, ns)

	results := d.check(context.Background(), ns)
	require.Len(t, results, 6)

	for _, result := range results {
		assert.True(t, result.ok, result.name+": "+result.detail)
	}

	var out bytes.Buffer
	d.print(&out, results)

	assert.Contains(t, out.String(), "PASS  Docker")
	assert.Contains(t, out.String(), "PASS  Ports 80/443")
	assert.Contains(t, out.String(), "PASS  DNS")
	assert.Contains(t, out.String(), "PASS  Proxy")
	assert.Contains(t, out.String(), "PASS  Disk")
	assert.Contains(t, out.String(), "PASS  Certificates")
	assert.Contains(t, out.String(), "All checks passed.")
}

func TestDoctorReportsDockerFailure(t *testing.T) {
	ns := doctorNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.example.com",
		Image: "example/app:latest",
	})
	d := passingDoctor(t, ns)
	d.ping = func(context.Context, *docker.Namespace) error {
		return docker.ErrDockerNotRunning
	}

	results := d.check(context.Background(), ns)
	require.Len(t, results, 6)

	assert.False(t, results[0].ok)
	assert.Equal(t, "Docker", results[0].name)
	assert.Equal(t, docker.ErrDockerNotRunning.Description(), results[0].detail)
	assert.NotEmpty(t, results[0].fix)

	for _, result := range results[1:] {
		assert.False(t, result.ok)
		assert.Contains(t, result.detail, "not checked")
	}
}

func TestDoctorReportsPortsInUse(t *testing.T) {
	ns := doctorNamespace(t)
	d := passingDoctor(t, ns)
	d.portAvailable = func(int) bool { return false }
	d.proxyStatus = func(context.Context, *docker.Proxy) (docker.ProxyStatus, error) {
		return docker.ProxyStatus{Exists: true, Running: true}, nil
	}

	result := d.checkPorts(context.Background(), ns)

	assert.False(t, result.ok)
	assert.Contains(t, result.detail, "80 and 443")
	assert.Contains(t, result.fix, "Stop the service using port 80 and 443")
}

func TestDoctorReportsUnresolvedHosts(t *testing.T) {
	ns := doctorNamespace(t,
		docker.ApplicationSettings{Name: "good", Host: "good.example.com", Image: "example/good:latest"},
		docker.ApplicationSettings{Name: "bad", Host: "bad.example.com", Image: "example/bad:latest"},
	)
	d := passingDoctor(t, ns)
	d.lookupHost = func(_ context.Context, host string) ([]string, error) {
		if host == "bad.example.com" {
			return nil, errors.New("no such host")
		}
		return []string{"192.0.2.1"}, nil
	}

	result := d.checkDNS(context.Background(), ns)

	assert.False(t, result.ok)
	assert.Equal(t, "bad.example.com does not resolve", result.detail)
	assert.Contains(t, result.fix, "Add an A or AAAA record for bad.example.com")
}

func TestDoctorReportsStoppedProxy(t *testing.T) {
	ns := doctorNamespace(t)
	d := passingDoctor(t, ns)
	d.proxyStatus = func(context.Context, *docker.Proxy) (docker.ProxyStatus, error) {
		return docker.ProxyStatus{Exists: true, Running: false}, nil
	}

	result := d.checkProxy(context.Background(), ns)

	assert.False(t, result.ok)
	assert.Equal(t, "container test-proxy is not running", result.detail)
	assert.Contains(t, result.fix, "docker start test-proxy")
}

func TestDoctorReportsLowDisk(t *testing.T) {
	ns := doctorNamespace(t)
	d := passingDoctor(t, ns)
	d.diskSpace = func(string) (system.DiskSpace, error) {
		return system.DiskSpace{Total: 10_000_000_000, Free: 1_000_000_000}, nil
	}

	result := d.checkDisk(context.Background(), ns)

	assert.False(t, result.ok)
	assert.Equal(t, "1GB free of 10GB (10%)", result.detail)
	assert.Contains(t, result.fix, "docker system prune")
}

func TestDoctorReportsExpiringCertificate(t *testing.T) {
	ns := doctorNamespace(t, docker.ApplicationSettings{
		Name:  "myapp",
		Host:  "myapp.example.com",
		Image: "example/app:latest",
	})
	d := passingDoctor(t, ns)
	d.certificateExpiry = func(context.Context, string, int) (time.Time, error) {
		return d.now().Add(10 * 24 * time.Hour), nil
	}

	result := d.checkCertificates(context.Background(), ns)

	assert.False(t, result.ok)
	assert.Contains(t, result.detail, "myapp.example.com expires")
	assert.Contains(t, result.detail, "(10 days)")
	assert.Contains(t, result.fix, "Restart the proxy")
}

func TestDoctorRunReturnsFailureWhenChecksFail(t *testing.T) {
	ns := doctorNamespace(t)
	d := passingDoctor(t, ns)
	d.ping = func(context.Context, *docker.Namespace) error {
		return docker.ErrDockerNotRunning
	}

	var out bytes.Buffer
	d.cmd.SetOut(&out)

	err := d.run(context.Background(), ns, d.cmd, nil)
	require.Error(t, err)

	code, ok := ExitCode(err)
	require.True(t, ok)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "FAIL  Docker")
	assert.Contains(t, out.String(), "6 check(s) failed.")
}

func TestDoctorPrintIncludesFixForFailures(t *testing.T) {
	var out bytes.Buffer
	d := newDoctorCommand()

	d.print(&out, []doctorResult{{
		name:   "Ports 80/443",
		detail: "port 80 is already in use",
		fix:    "stop the service using it",
	}})

	assert.Contains(t, out.String(), "FAIL  Ports 80/443")
	assert.Contains(t, out.String(), "(fix: stop the service using it)")
	assert.Contains(t, out.String(), "1 check(s) failed.")
}

// Helpers

func doctorNamespace(t *testing.T, apps ...docker.ApplicationSettings) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(apps...))
	require.NoError(t, err)

	return ns
}

func passingDoctor(t *testing.T, ns *docker.Namespace) *doctorCommand {
	t.Helper()

	d := newDoctorCommand()
	d.now = func() time.Time {
		return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	}
	d.ping = func(context.Context, *docker.Namespace) error {
		return nil
	}
	d.refresh = func(context.Context, *docker.Namespace) error {
		return nil
	}
	d.proxyStatus = func(context.Context, *docker.Proxy) (docker.ProxyStatus, error) {
		return docker.ProxyStatus{
			Exists:         true,
			Running:        true,
			PublishedPorts: map[int]bool{80: true, 443: true},
		}, nil
	}
	d.portAvailable = func(int) bool {
		return false
	}
	d.lookupHost = func(context.Context, string) ([]string, error) {
		return []string{"192.0.2.1"}, nil
	}
	d.certificateExpiry = func(context.Context, string, int) (time.Time, error) {
		return d.now().Add(60 * 24 * time.Hour), nil
	}
	d.dockerRootDir = func(context.Context, *docker.Namespace) (string, error) {
		return "/var/lib/docker", nil
	}
	d.diskSpace = func(string) (system.DiskSpace, error) {
		return system.DiskSpace{Total: 100_000_000_000, Free: 50_000_000_000}, nil
	}

	return d
}
