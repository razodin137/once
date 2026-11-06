package command

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/system"
)

const (
	doctorCheckTimeout         = 5 * time.Second
	doctorFreeDiskThreshold    = 5 << 30
	doctorCertificateWarning   = 14 * 24 * time.Hour
	doctorResultNameWidth      = 14
	doctorResultStatusWidth    = 4
	doctorDefaultDockerDataDir = "/var/lib/docker"
)

type doctorCommand struct {
	cmd *cobra.Command

	now               func() time.Time
	newNamespace      func(string) (*docker.Namespace, error)
	ping              func(context.Context, *docker.Namespace) error
	refresh           func(context.Context, *docker.Namespace) error
	proxyStatus       func(context.Context, *docker.Proxy) (docker.ProxyStatus, error)
	portAvailable     func(int) bool
	lookupHost        func(context.Context, string) ([]string, error)
	certificateExpiry func(context.Context, string, int) (time.Time, error)
	dockerRootDir     func(context.Context, *docker.Namespace) (string, error)
	diskSpace         func(string) (system.DiskSpace, error)
}

type doctorResult struct {
	name   string
	ok     bool
	detail string
	fix    string
}

type doctorFailure struct {
	count int
}

func newDoctorCommand() *doctorCommand {
	d := &doctorCommand{
		now: time.Now,
		newNamespace: func(name string) (*docker.Namespace, error) {
			return docker.NewNamespace(name)
		},
		ping: func(ctx context.Context, ns *docker.Namespace) error {
			return ns.Ping(ctx)
		},
		refresh: func(ctx context.Context, ns *docker.Namespace) error {
			return ns.Refresh(ctx)
		},
		proxyStatus: func(ctx context.Context, proxy *docker.Proxy) (docker.ProxyStatus, error) {
			return proxy.Status(ctx)
		},
		portAvailable:     portAvailable,
		lookupHost:        net.DefaultResolver.LookupHost,
		certificateExpiry: certificateExpiry,
		dockerRootDir: func(ctx context.Context, ns *docker.Namespace) (string, error) {
			return ns.DockerRootDir(ctx)
		},
		diskSpace: system.DiskSpaceAt,
	}

	d.cmd = &cobra.Command{
		Use:   "doctor",
		Short: "Check that Once can serve applications",
		Args:  cobra.NoArgs,
		RunE:  d.execute,
	}

	return d
}

func (e doctorFailure) Error() string {
	return fmt.Sprintf("%d check(s) failed", e.count)
}

func (e doctorFailure) ExitCode() int {
	return 1
}

// Private

func (d *doctorCommand) execute(cmd *cobra.Command, args []string) error {
	ns, err := d.newNamespace(namespaceFlag(cmd))
	if err != nil {
		return fmt.Errorf("creating namespace: %w", err)
	}

	return d.run(context.Background(), ns, cmd, args)
}

func (d *doctorCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	results := d.check(ctx, ns)
	d.print(cmd.OutOrStdout(), results)

	if failed := failedCheckCount(results); failed > 0 {
		return doctorFailure{count: failed}
	}

	return nil
}

func (d *doctorCommand) check(ctx context.Context, ns *docker.Namespace) []doctorResult {
	dockerResult := d.checkDocker(ctx, ns)
	if !dockerResult.ok {
		return append([]doctorResult{dockerResult}, unavailableCheckResults()...)
	}

	if err := d.refresh(ctx, ns); err != nil {
		dockerResult.ok = false
		dockerResult.detail = "connected, but could not load applications: " + err.Error()
		dockerResult.fix = "Check the Docker daemon and try again."
		return append([]doctorResult{dockerResult}, unavailableCheckResults()...)
	}

	return []doctorResult{
		dockerResult,
		d.checkPorts(ctx, ns),
		d.checkDNS(ctx, ns),
		d.checkProxy(ctx, ns),
		d.checkDisk(ctx, ns),
		d.checkCertificates(ctx, ns),
	}
}

func (d *doctorCommand) checkDocker(ctx context.Context, ns *docker.Namespace) doctorResult {
	if err := d.ping(ctx, ns); err != nil {
		return doctorResult{
			name:   "Docker",
			detail: docker.ErrorMessage(err),
			fix:    dockerFix(err),
		}
	}

	return doctorResult{name: "Docker", ok: true, detail: "daemon is reachable"}
}

func (d *doctorCommand) checkPorts(ctx context.Context, ns *docker.Namespace) doctorResult {
	status, err := d.proxyStatus(ctx, ns.Proxy())
	if err != nil {
		return doctorResult{
			name:   "Ports 80/443",
			detail: "could not inspect the proxy: " + err.Error(),
			fix:    "Fix the Docker connection, then run `once doctor` again.",
		}
	}

	var busy []string
	for _, port := range []int{docker.DefaultHTTPPort, docker.DefaultHTTPSPort} {
		if d.portAvailable(port) {
			continue
		}
		if status.Running && status.PublishedPorts[port] {
			continue
		}
		busy = append(busy, strconv.Itoa(port))
	}

	if len(busy) == 0 {
		return doctorResult{name: "Ports 80/443", ok: true, detail: "available"}
	}

	return doctorResult{
		name:   "Ports 80/443",
		detail: "port " + strings.Join(busy, " and ") + " is already in use",
		fix:    "Stop the service using port " + strings.Join(busy, " and ") + ", then run `once doctor` again.",
	}
}

func (d *doctorCommand) checkDNS(ctx context.Context, ns *docker.Namespace) doctorResult {
	apps := ns.Applications()
	if len(apps) == 0 {
		return doctorResult{name: "DNS", ok: true, detail: "no application hosts to check"}
	}

	var failed []string
	for _, app := range apps {
		lookupCtx, cancel := context.WithTimeout(ctx, doctorCheckTimeout)
		_, err := d.lookupHost(lookupCtx, app.Settings.Host)
		cancel()
		if err != nil {
			failed = append(failed, app.Settings.Host)
		}
	}

	if len(failed) == 0 {
		return doctorResult{
			name:   "DNS",
			ok:     true,
			detail: fmt.Sprintf("%d host(s) resolve", len(apps)),
		}
	}

	detail := failed[0] + " does not resolve"
	if len(failed) > 1 {
		detail = fmt.Sprintf("%s and %d more do not resolve", failed[0], len(failed)-1)
	}

	return doctorResult{
		name:   "DNS",
		detail: detail,
		fix:    "Add an A or AAAA record for " + strings.Join(failed, ", ") + " pointing to this server.",
	}
}

func (d *doctorCommand) checkProxy(ctx context.Context, ns *docker.Namespace) doctorResult {
	status, err := d.proxyStatus(ctx, ns.Proxy())
	if err != nil {
		return doctorResult{
			name:   "Proxy",
			detail: "could not inspect the proxy container: " + err.Error(),
			fix:    "Fix the Docker connection, then run `once doctor` again.",
		}
	}

	name := ns.Name() + "-proxy"
	if !status.Exists {
		return doctorResult{
			name:   "Proxy",
			detail: "container " + name + " was not found",
			fix:    "Run `once` and deploy an application to create the proxy.",
		}
	}

	if !status.Running {
		return doctorResult{
			name:   "Proxy",
			detail: "container " + name + " is not running",
			fix:    "Start it with `docker start " + name + "`.",
		}
	}

	return doctorResult{name: "Proxy", ok: true, detail: "container " + name + " is running"}
}

func (d *doctorCommand) checkDisk(ctx context.Context, ns *docker.Namespace) doctorResult {
	path, err := d.dockerRootDir(ctx, ns)
	if err != nil || path == "" {
		path = doctorDefaultDockerDataDir
	}

	space, err := d.diskSpace(path)
	if err != nil {
		return doctorResult{
			name:   "Disk",
			detail: "could not read " + path + ": " + err.Error(),
			fix:    "Check that " + path + " exists and is readable.",
		}
	}

	detail := fmt.Sprintf("%s free of %s (%s)", formatDoctorBytes(space.Free), formatDoctorBytes(space.Total), doctorDiskPercent(space))
	if space.Free < doctorFreeDiskThreshold {
		return doctorResult{
			name:   "Disk",
			detail: detail,
			fix:    "Free space in " + path + " by removing unused images and containers with `docker system prune`.",
		}
	}

	return doctorResult{name: "Disk", ok: true, detail: detail}
}

func (d *doctorCommand) checkCertificates(ctx context.Context, ns *docker.Namespace) doctorResult {
	var apps []*docker.Application
	for _, app := range ns.Applications() {
		if app.Settings.TLSEnabled() {
			apps = append(apps, app)
		}
	}

	if len(apps) == 0 {
		return doctorResult{name: "Certificates", ok: true, detail: "no TLS hosts to check"}
	}

	port := docker.DefaultHTTPSPort
	if settings := ns.Proxy().Settings; settings != nil && settings.HTTPSPort != 0 {
		port = settings.HTTPSPort
	}

	var earliest time.Time
	var earliestHost string
	var failed []string

	for _, app := range apps {
		expiryCtx, cancel := context.WithTimeout(ctx, doctorCheckTimeout)
		expiry, err := d.certificateExpiry(expiryCtx, app.Settings.Host, port)
		cancel()
		if err != nil {
			failed = append(failed, app.Settings.Host)
			continue
		}
		if earliest.IsZero() || expiry.Before(earliest) {
			earliest = expiry
			earliestHost = app.Settings.Host
		}
	}

	if len(failed) > 0 {
		detail := "could not read a certificate for " + failed[0]
		if len(failed) > 1 {
			detail = fmt.Sprintf("could not read certificates for %d hosts (first: %s)", len(failed), failed[0])
		}
		return doctorResult{
			name:   "Certificates",
			detail: detail,
			fix:    "Check DNS and ports 80/443, then restart the proxy so it can request a certificate.",
		}
	}

	days := int(earliest.Sub(d.now()).Hours() / 24)
	detail := fmt.Sprintf("%s expires %s (%d days)", earliestHost, earliest.Format("2 Jan 2006"), days)

	if days < 0 {
		return doctorResult{
			name:   "Certificates",
			detail: fmt.Sprintf("%s expired %s", earliestHost, earliest.Format("2 Jan 2006")),
			fix:    "Restart the proxy container so it can renew the certificate.",
		}
	}

	if days < int(doctorCertificateWarning.Hours()/24) {
		return doctorResult{
			name:   "Certificates",
			detail: detail,
			fix:    "Restart the proxy container so it can renew the certificate.",
		}
	}

	return doctorResult{name: "Certificates", ok: true, detail: detail}
}

func (d *doctorCommand) print(w io.Writer, results []doctorResult) {
	for _, result := range results {
		status := "PASS"
		if !result.ok {
			status = "FAIL"
		}

		detail := result.detail
		if !result.ok && result.fix != "" {
			detail += " (fix: " + result.fix + ")"
		}

		fmt.Fprintf(w, "%-*s  %-*s  %s\n", doctorResultStatusWidth, status, doctorResultNameWidth, result.name, detail)
	}

	failed := failedCheckCount(results)
	if failed == 0 {
		fmt.Fprintln(w, "\nAll checks passed.")
	} else {
		fmt.Fprintf(w, "\n%d check(s) failed.\n", failed)
	}
}

// Helpers

func unavailableCheckResults() []doctorResult {
	names := []string{"Ports 80/443", "DNS", "Proxy", "Disk", "Certificates"}
	results := make([]doctorResult, len(names))
	for i, name := range names {
		results[i] = doctorResult{
			name:   name,
			detail: "not checked because Docker is unavailable",
			fix:    "Fix the Docker check first, then run `once doctor` again.",
		}
	}
	return results
}

func failedCheckCount(results []doctorResult) int {
	failed := 0
	for _, result := range results {
		if !result.ok {
			failed++
		}
	}
	return failed
}

func dockerFix(err error) string {
	if errors.Is(err, docker.ErrDockerPermissionDenied) {
		return "Run `once doctor` with sudo, or add your user to the `docker` group."
	}
	return "Install Docker and start the Docker daemon."
}

func portAvailable(port int) bool {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err == nil {
		listener.Close()
		return true
	}

	// Without privilege to bind a low port we can't tell whether it is free
	// from the bind error alone, so look for a listener instead.
	if errors.Is(err, os.ErrPermission) {
		return !portListening(port)
	}

	return false
}

func portListening(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func certificateExpiry(ctx context.Context, host string, port int) (time.Time, error) {
	dialer := &net.Dialer{Timeout: doctorCheckTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, strconv.Itoa(port)), &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	})
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, errors.New("no peer certificate")
	}

	return certs[0].NotAfter, nil
}

func formatDoctorBytes(bytes uint64) string {
	const (
		kb = 1_000
		mb = 1_000 * kb
		gb = 1_000 * mb
		tb = 1_000 * gb
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1fTB", float64(bytes)/tb)
	case bytes >= gb:
		return fmt.Sprintf("%.0fGB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.0fMB", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.0fKB", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func doctorDiskPercent(space system.DiskSpace) string {
	if space.Total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", float64(space.Free)/float64(space.Total)*100)
}
