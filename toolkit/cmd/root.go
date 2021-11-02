package cmd

import (
	"log"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var rootCmd = &cobra.Command{
	Use:   "aws-basics",
	Short: "Tools for interacting with aws-basics systems",
}

func prerequisites() error {
	g := new(errgroup.Group)

	g.Go(func() error {
		err := exec.Command("which", "docker").Run()
		return errors.Wrap(err, "docker is not available")
	})

	g.Go(func() error {
		err := exec.Command("which", "npm").Run()
		return errors.Wrap(err, "npm is not available")
	})

	g.Go(func() error {
		err := exec.Command("which", "git").Run()
		return errors.Wrap(err, "git is not available")
	})

	g.Go(func() error {
		err := exec.Command("which", "make").Run()
		return errors.Wrap(err, "make is not available")
	})

	g.Go(func() error {
		err := exec.Command("which", "aws").Run()
		return errors.Wrap(err, "aws-cli is not available")
	})

	g.Go(func() error {
		err := exec.Command("which", "zip").Run()
		return errors.Wrap(err, "zip is not available")
	})

	return g.Wait()
}

// Execute runs the CLI.
func Execute() {
	if err := prerequisites(); err != nil {
		log.Fatal(errors.Wrap(err, "prerequisites not met"))
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
