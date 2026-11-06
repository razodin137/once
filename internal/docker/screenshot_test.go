package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureScreenshotStoresPNG(t *testing.T) {
	app := screenshotApplication(t)
	var capturedURL string
	app.capture = func(_ context.Context, pageURL string) ([]byte, error) {
		capturedURL = pageURL
		return testPNG("page"), nil
	}

	require.NoError(t, app.CaptureScreenshot(context.Background()))

	assert.Equal(t, "http://myapp.example.com", capturedURL)
	assert.True(t, app.HasScreenshot())

	path, err := app.ScreenshotPath()
	require.NoError(t, err)
	assert.Equal(t, testPNG("page"), mustReadFile(t, path))
}

func TestCaptureScreenshotWithoutHost(t *testing.T) {
	app := &Application{
		Settings:      ApplicationSettings{Name: "myapp"},
		screenshotDir: t.TempDir(),
	}
	app.capture = func(context.Context, string) ([]byte, error) {
		t.Fatal("should not capture without a host")
		return nil, nil
	}

	err := app.CaptureScreenshot(context.Background())

	require.ErrorIs(t, err, ErrHostRequired)
	assert.False(t, app.HasScreenshot())
}

func TestCaptureScreenshotRejectsNonPNG(t *testing.T) {
	app := screenshotApplication(t)
	app.capture = func(context.Context, string) ([]byte, error) {
		return []byte("not a png"), nil
	}

	err := app.CaptureScreenshot(context.Background())

	require.ErrorIs(t, err, ErrScreenshotFailed)
	assert.False(t, app.HasScreenshot())
}

func TestCaptureScreenshotReturnsCaptureError(t *testing.T) {
	app := screenshotApplication(t)
	app.capture = func(context.Context, string) ([]byte, error) {
		return nil, errors.New("browser crashed")
	}

	err := app.CaptureScreenshot(context.Background())

	require.ErrorContains(t, err, "browser crashed")
	assert.False(t, app.HasScreenshot())
}

func TestScreenshotReadsStoredFile(t *testing.T) {
	app := screenshotApplication(t)
	path, err := app.ScreenshotPath()
	require.NoError(t, err)

	require.NoError(t, writeScreenshot(path, testPNG("stored")))

	png, err := app.Screenshot()
	require.NoError(t, err)
	assert.Equal(t, testPNG("stored"), png)
}

func TestScreenshotMissingFile(t *testing.T) {
	app := screenshotApplication(t)

	_, err := app.Screenshot()
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestScreenshotHost(t *testing.T) {
	t.Run("includes only the hostname", func(t *testing.T) {
		host, err := screenshotHost("https://myapp.example.com:8443/page")
		require.NoError(t, err)
		assert.Equal(t, "myapp.example.com", host)
	})

	t.Run("rejects a URL without a host", func(t *testing.T) {
		_, err := screenshotHost("not a url")
		require.ErrorIs(t, err, ErrScreenshotFailed)
	})
}

func TestScreenshotInternalURL(t *testing.T) {
	t.Run("keeps a URL without a published port", func(t *testing.T) {
		pageURL, err := screenshotInternalURL("https://myapp.example.com/page")
		require.NoError(t, err)
		assert.Equal(t, "https://myapp.example.com/page", pageURL)
	})

	t.Run("drops a published port", func(t *testing.T) {
		pageURL, err := screenshotInternalURL("https://myapp.example.com:8443/page")
		require.NoError(t, err)
		assert.Equal(t, "https://myapp.example.com/page", pageURL)
	})
}

func TestWriteScreenshotCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "screen.png")

	require.NoError(t, writeScreenshot(path, testPNG("nested")))

	assert.Equal(t, testPNG("nested"), mustReadFile(t, path))
}

// Helpers

func screenshotApplication(t *testing.T) *Application {
	t.Helper()

	return &Application{
		Settings: ApplicationSettings{
			Name:       "myapp",
			Host:       "myapp.example.com",
			DisableTLS: true,
		},
		screenshotDir: t.TempDir(),
	}
}

func testPNG(content string) []byte {
	return append(append([]byte{}, pngSignature...), []byte(content)...)
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
