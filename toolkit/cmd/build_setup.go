package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/rclark/aws-basics/toolkit/src/configuration"
	"github.com/spf13/cobra"
)

var buildSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure a repository for builds",
	Run: func(cmd *cobra.Command, args []string) {
		builds := &configuration.Builds{}
		if err := builds.Prompt(); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				return
			}

			log.Fatal(err)
		}

		dir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		if err := configuration.AddOrReplace(dir, builds); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				return
			}

			log.Fatal(err)
		}
	},
}

func init() {
	buildCmd.AddCommand(buildSetupCmd)
}
