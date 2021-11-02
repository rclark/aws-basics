package cmd

import "github.com/spf13/cobra"

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Tools for building artifacts to deploy on AWS",
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
