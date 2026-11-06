package command

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/basecamp/once/internal/docker"
)

type fakeLogStream struct {
	lines   []docker.LogLine
	version uint64
	started string
	stopped bool
	onStart func()
}

func (f *fakeLogStream) Start(_ context.Context, containerName string) {
	f.started = containerName
	f.version = uint64(len(f.lines))
	if f.onStart != nil {
		f.onStart()
	}
}

func (f *fakeLogStream) Stop() {
	f.stopped = true
}

func (f *fakeLogStream) FetchSince(version uint64) ([]docker.LogLine, uint64) {
	if f.version <= version {
		return nil, f.version
	}

	lines := f.lines
	if pending := int(f.version - version); pending < len(lines) {
		lines = lines[len(lines)-pending:]
	}

	return lines, f.version
}

func TestLogsStreamsApplicationLogs(t *testing.T) {
	ns := logsTestNamespace(t)
	stream := &fakeLogStream{lines: []docker.LogLine{
		{Content: "hello"},
		{Content: "uh oh", IsStderr: true},
	}}

	l := newLogsCommand()
	var settings docker.LogStreamerSettings
	l.containerName = func(context.Context, *docker.Application) (string, error) {
		return "once-app-myapp-abc", nil
	}
	l.newStreamer = func(_ *docker.Namespace, s docker.LogStreamerSettings) logStream {
		settings = s
		return stream
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream.onStart = cancel

	var stdout, stderr bytes.Buffer
	l.cmd.SetOut(&stdout)
	l.cmd.SetErr(&stderr)

	err := l.run(ctx, ns, l.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	assert.Equal(t, "hello\n", stdout.String())
	assert.Equal(t, "uh oh\n", stderr.String())
	assert.Equal(t, "once-app-myapp-abc", stream.started)
	assert.True(t, stream.stopped)
	assert.Equal(t, 100, settings.Tail)
}

func TestLogsGrepFiltersLines(t *testing.T) {
	ns := logsTestNamespace(t)
	stream := &fakeLogStream{lines: []docker.LogLine{
		{Content: "GET /"},
		{Content: "POST /"},
		{Content: "GET /health"},
	}}

	l := newLogsCommand()
	l.grep = "GET"
	l.containerName = func(context.Context, *docker.Application) (string, error) {
		return "once-app-myapp-abc", nil
	}
	l.newStreamer = func(*docker.Namespace, docker.LogStreamerSettings) logStream {
		return stream
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream.onStart = cancel

	var stdout bytes.Buffer
	l.cmd.SetOut(&stdout)
	l.cmd.SetErr(io.Discard)

	err := l.run(ctx, ns, l.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)

	assert.Equal(t, "GET /\nGET /health\n", stdout.String())
}

func TestLogsPassesTailSetting(t *testing.T) {
	ns := logsTestNamespace(t)
	stream := &fakeLogStream{}

	l := newLogsCommand()
	l.tail = 25
	var settings docker.LogStreamerSettings
	l.containerName = func(context.Context, *docker.Application) (string, error) {
		return "once-app-myapp-abc", nil
	}
	l.newStreamer = func(_ *docker.Namespace, s docker.LogStreamerSettings) logStream {
		settings = s
		return stream
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream.onStart = cancel

	l.cmd.SetOut(io.Discard)
	l.cmd.SetErr(io.Discard)

	err := l.run(ctx, ns, l.cmd, []string{"myapp.localhost"})
	require.NoError(t, err)
	assert.Equal(t, 25, settings.Tail)
}

func TestLogsRejectsInvalidTail(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	l := newLogsCommand()
	l.tail = 0

	err = l.run(context.Background(), ns, l.cmd, []string{"myapp.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tail must be at least 1")
}

func TestLogsRejectsInvalidGrep(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	l := newLogsCommand()
	l.grep = "["

	err = l.run(context.Background(), ns, l.cmd, []string{"myapp.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid grep pattern")
}

func TestLogsUnknownHost(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	l := newLogsCommand()

	err = l.run(context.Background(), ns, l.cmd, []string{"missing.localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestPrintLogLinesOnlyPrintsNewLines(t *testing.T) {
	stream := &fakeLogStream{lines: []docker.LogLine{
		{Content: "one"},
		{Content: "two"},
	}}
	stream.version = 2

	var out bytes.Buffer
	printed := printLogLines(&out, io.Discard, stream, 0, nil)
	assert.Equal(t, "one\ntwo\n", out.String())
	assert.Equal(t, uint64(2), printed)

	stream.lines = append(stream.lines, docker.LogLine{Content: "three"})
	stream.version = 3

	printed = printLogLines(&out, io.Discard, stream, printed, nil)
	assert.Equal(t, "one\ntwo\nthree\n", out.String())
	assert.Equal(t, uint64(3), printed)
}

// Helpers

func logsTestNamespace(t *testing.T) *docker.Namespace {
	t.Helper()

	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
	))
	require.NoError(t, err)

	return ns
}
