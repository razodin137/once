package browser

import (
	"errors"
	"os/exec"
	"runtime"
)

func Open(url string) error {
	if url == "" {
		return errors.New("url is required")
	}

	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}

	return exec.Command(name, url).Run()
}
