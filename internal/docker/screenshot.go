package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	ScreenshotImage = "zenika/alpine-chrome:with-puppeteer"

	screenshotContainerPath = "/tmp/screenshot.png"
	screenshotFileName      = "screenshot.png"
	screenshotPullTimeout   = 10 * time.Minute
	screenshotRunTimeout    = 2 * time.Minute
)

const screenshotScript = `const puppeteer = require("puppeteer")

const main = async () => {
  const [ pageURL, outputPath, host, proxyIP ] = process.argv.slice(1)
  const browser = await puppeteer.launch({
    args: [
      "--no-sandbox",
      "--disable-setuid-sandbox",
      "--host-resolver-rules=MAP " + host + " " + proxyIP
    ]
  })
  try {
    const page = await browser.newPage()
    await page.setViewport({ width: 1280, height: 800 })
    await page.goto(pageURL, { waitUntil: "networkidle2", timeout: 30000 })
    await page.screenshot({ path: outputPath, fullPage: true })
  } finally {
    await browser.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
`

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// CaptureScreenshot takes a full-page screenshot of the application's own URL
// using a throwaway headless browser container and stores it alongside the
// application's record so later views can show it.
func (a *Application) CaptureScreenshot(ctx context.Context) error {
	err := a.captureScreenshot(ctx)
	if a.namespace != nil {
		a.saveOperationResult(ctx, func(s *State) { s.RecordScreenshot(a.Settings.Name, err) })
	}
	return err
}

func (a *Application) HasScreenshot() bool {
	path, err := a.ScreenshotPath()
	if err != nil {
		return false
	}

	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (a *Application) Screenshot() ([]byte, error) {
	path, err := a.ScreenshotPath()
	if err != nil {
		return nil, err
	}

	png, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading screenshot: %w", err)
	}
	return png, nil
}

func (a *Application) ScreenshotPath() (string, error) {
	dir, err := a.screenshotDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, screenshotFileName), nil
}

// Private

func (a *Application) captureScreenshot(ctx context.Context) error {
	pageURL := a.URL()
	if pageURL == "" {
		return ErrHostRequired
	}

	capture := a.capture
	if capture == nil {
		capture = a.captureWithContainer
	}

	png, err := capture(ctx, pageURL)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(png, pngSignature) {
		return fmt.Errorf("%w: the browser did not return a PNG", ErrScreenshotFailed)
	}

	path, err := a.ScreenshotPath()
	if err != nil {
		return err
	}

	if err := writeScreenshot(path, png); err != nil {
		return err
	}

	slog.Info("Captured screenshot", "app", a.Settings.Name, "path", path)
	return nil
}

func (a *Application) captureWithContainer(ctx context.Context, pageURL string) ([]byte, error) {
	proxyIP, err := a.proxyIP(ctx)
	if err != nil {
		return nil, err
	}

	host, err := screenshotHost(pageURL)
	if err != nil {
		return nil, err
	}

	internalURL, err := screenshotInternalURL(pageURL)
	if err != nil {
		return nil, err
	}

	if err := a.ensureScreenshotImage(ctx); err != nil {
		return nil, err
	}

	id, err := ContainerRandomID()
	if err != nil {
		return nil, fmt.Errorf("generating screenshot container id: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, screenshotRunTimeout)
	defer cancel()

	resp, err := a.namespace.client.ContainerCreate(runCtx, client.ContainerCreateOptions{
		Name: fmt.Sprintf("%s-screenshot-%s", a.namespace.name, id),
		Config: &container.Config{
			Image: ScreenshotImage,
			Cmd:   []string{"node", "-e", screenshotScript, internalURL, screenshotContainerPath, host, proxyIP},
		},
		HostConfig: &container.HostConfig{
			AutoRemove: false,
			ExtraHosts: []string{host + ":" + proxyIP},
			IpcMode:    container.IPCModeHost,
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				a.namespace.name: {},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: creating browser container: %w", ErrScreenshotFailed, err)
	}

	defer func() {
		removeCtx, cancel := context.WithTimeout(context.WithoutCancel(runCtx), 30*time.Second)
		defer cancel()
		if _, err := a.namespace.client.ContainerRemove(removeCtx, resp.ID, client.ContainerRemoveOptions{Force: true}); err != nil {
			slog.Warn("Failed to remove screenshot container", "app", a.Settings.Name, "container", resp.ID, "error", err)
		}
	}()

	if _, err := a.namespace.client.ContainerStart(runCtx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("%w: starting browser container: %w", ErrScreenshotFailed, err)
	}

	if err := a.waitForScreenshotContainer(runCtx, resp.ID); err != nil {
		return nil, err
	}

	png, err := a.copyScreenshotFromContainer(runCtx, resp.ID)
	if err != nil {
		return nil, err
	}

	return png, nil
}

func (a *Application) waitForScreenshotContainer(ctx context.Context, containerID string) error {
	wait := a.namespace.client.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: browser container timed out", ErrScreenshotFailed)
	case err := <-wait.Error:
		return fmt.Errorf("%w: waiting for browser container: %w", ErrScreenshotFailed, err)
	case result := <-wait.Result:
		if result.StatusCode != 0 {
			output := a.containerOutput(ctx, containerID)
			if output == "" {
				return fmt.Errorf("%w: browser container exited with status %d", ErrScreenshotFailed, result.StatusCode)
			}
			return fmt.Errorf("%w: browser container exited with status %d: %s", ErrScreenshotFailed, result.StatusCode, output)
		}
		return nil
	}
}

func (a *Application) copyScreenshotFromContainer(ctx context.Context, containerID string) ([]byte, error) {
	res, err := a.namespace.client.CopyFromContainer(ctx, containerID, client.CopyFromContainerOptions{
		SourcePath: screenshotContainerPath,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: copying screenshot from browser container: %w", ErrScreenshotFailed, err)
	}
	defer res.Content.Close()

	tr := tar.NewReader(res.Content)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: reading screenshot archive: %w", ErrScreenshotFailed, err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		png, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("%w: reading screenshot: %w", ErrScreenshotFailed, err)
		}
		return png, nil
	}

	return nil, fmt.Errorf("%w: no screenshot file in browser container", ErrScreenshotFailed)
}

func (a *Application) containerOutput(ctx context.Context, containerID string) string {
	reader, err := a.namespace.client.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return ""
	}
	defer reader.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String() + "\n" + stderr.String())
}

func (a *Application) ensureScreenshotImage(ctx context.Context) error {
	dockerClient := a.namespace.client

	if _, err := dockerClient.ImageInspect(ctx, ScreenshotImage); err == nil {
		return nil
	} else if !errdefs.IsNotFound(err) {
		return fmt.Errorf("%w: inspecting browser image: %w", ErrScreenshotFailed, err)
	}

	pullCtx, cancel := context.WithTimeout(ctx, screenshotPullTimeout)
	defer cancel()

	reader, err := dockerClient.ImagePull(pullCtx, ScreenshotImage, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("%w: pulling browser image: %w", ErrScreenshotFailed, err)
	}
	defer reader.Close()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("%w: pulling browser image: %w", ErrScreenshotFailed, err)
	}

	return nil
}

func (a *Application) proxyIP(ctx context.Context) (string, error) {
	info, err := a.namespace.client.ContainerInspect(ctx, a.namespace.Proxy().containerName(), client.ContainerInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("%w: inspecting proxy container: %w", ErrScreenshotFailed, err)
	}

	if info.Container.NetworkSettings == nil {
		return "", fmt.Errorf("%w: the proxy container has no network settings", ErrScreenshotFailed)
	}

	endpoint := info.Container.NetworkSettings.Networks[a.namespace.name]
	if endpoint == nil || !endpoint.IPAddress.IsValid() {
		return "", fmt.Errorf("%w: the proxy container is not on the %s network", ErrScreenshotFailed, a.namespace.name)
	}

	return endpoint.IPAddress.String(), nil
}

func (a *Application) screenshotDirectory() (string, error) {
	if a.screenshotDir != "" {
		return a.screenshotDir, nil
	}

	if a.Settings.Name == "" {
		return "", fmt.Errorf("application name is required")
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("determining cache directory: %w", err)
	}

	return filepath.Join(cache, "once", "screenshots", a.Settings.Name), nil
}

// Helpers

func screenshotHost(pageURL string) (string, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("%w: parsing application URL: %w", ErrScreenshotFailed, err)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("%w: application URL has no host", ErrScreenshotFailed)
	}

	return host, nil
}

// screenshotInternalURL drops a published port, since the browser container
// reaches the proxy on its internal HTTP or HTTPS port rather than the port
// published on the host.
func screenshotInternalURL(pageURL string) (string, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("%w: parsing application URL: %w", ErrScreenshotFailed, err)
	}

	if parsed.Port() == "" {
		return pageURL, nil
	}

	parsed.Host = parsed.Hostname()
	return parsed.String(), nil
}

func writeScreenshot(path string, png []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating screenshot directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".screenshot-*.png")
	if err != nil {
		return fmt.Errorf("creating screenshot file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if _, err := file.Write(png); err != nil {
		file.Close()
		return fmt.Errorf("writing screenshot: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("writing screenshot: %w", err)
	}

	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("setting screenshot permissions: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("saving screenshot: %w", err)
	}

	return nil
}
