package command

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/docker"
)

type deployCommand struct {
	cmd   *cobra.Command
	flags settingsFlags
}

func newDeployCommand() *deployCommand {
	d := &deployCommand{}
	d.cmd = &cobra.Command{
		Use:   "deploy <image|path>",
		Short: "Deploy an application or static site",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(d.run),
	}

	d.flags.register(d.cmd)

	return d
}

// Private

func (d *deployCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	imageRef, source := deployTarget(args[0])

	if err := ns.Setup(ctx); err != nil {
		return fmt.Errorf("%w: %w", docker.ErrSetupFailed, err)
	}

	baseName := docker.NameFromImageRef(imageRef)
	if source != "" {
		baseName = docker.NameFromPath(source)
	}

	host := d.flags.host
	if host == "" {
		host = baseName + ".localhost"
	}

	existing, err := siteToReplace(ns, host, source)
	if err != nil {
		return err
	}
	if existing != nil {
		return d.redeploy(ctx, cmd, existing, imageRef, source)
	}

	settings, err := d.flags.buildSettings(cmd, imageRef, host)
	if err != nil {
		return err
	}
	settings.Source = source

	name, err := ns.UniqueName(baseName)
	if err != nil {
		return fmt.Errorf("generating app name: %w", err)
	}
	settings.Name = name

	app := docker.NewApplication(ns, settings)

	return runWithProgress("Deploying "+host, func(progress docker.DeployProgressCallback) error {
		if err := app.Deploy(ctx, progress); err != nil {
			if cleanupErr := app.Destroy(context.Background(), true); cleanupErr != nil {
				slog.Error("Failed to clean up after deploy failure", "app", name, "error", cleanupErr)
			}
			return fmt.Errorf("%w: %w", docker.ErrDeployFailed, err)
		}

		if err := app.VerifyHTTPOrRemove(ctx); err != nil {
			return err
		}

		return nil
	})
}

// redeploy replaces what an existing static site serves, keeping its host and
// settings. A failed redeploy leaves the previous version in place, so unlike
// a first deploy the app is never removed.
func (d *deployCommand) redeploy(ctx context.Context, cmd *cobra.Command, app *docker.Application, image, source string) error {
	settings, err := d.redeploySettings(cmd, app.Settings, image, source)
	if err != nil {
		return err
	}

	oldSettings := app.Settings
	app.Settings = settings

	return runWithProgress("Deploying "+settings.Host, func(progress docker.DeployProgressCallback) error {
		if err := app.Deploy(ctx, progress); err != nil {
			app.Settings = oldSettings
			return fmt.Errorf("%w: %w", docker.ErrDeployFailed, err)
		}

		return nil
	})
}

func (d *deployCommand) redeploySettings(cmd *cobra.Command, existing docker.ApplicationSettings, image, source string) (docker.ApplicationSettings, error) {
	settings, err := d.flags.applyChanges(cmd, existing, image)
	if err != nil {
		return docker.ApplicationSettings{}, err
	}

	settings.Source = source
	return settings, nil
}

// Helpers

// siteToReplace returns the static site already serving host, when deploying a
// folder should replace it in place. Deploying an image, or deploying a folder
// to a host owned by an application, is still a hostname conflict.
func siteToReplace(ns *docker.Namespace, host, source string) (*docker.Application, error) {
	existing := ns.ApplicationByHost(host)
	if existing == nil {
		return nil, nil
	}

	if source == "" || !existing.Settings.Static() {
		return nil, docker.ErrHostnameInUse
	}

	return existing, nil
}

func deployTarget(arg string) (imageRef, source string) {
	path, err := filepath.Abs(arg)
	if err != nil {
		return arg, ""
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return arg, ""
	}

	return docker.StaticImage, path
}
