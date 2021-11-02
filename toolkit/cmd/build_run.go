package cmd

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/toolkit/src/configuration"
	"github.com/rclark/aws-basics/toolkit/src/github"
	"github.com/spf13/cobra"
)

var buildRunCmd = &cobra.Command{
	Use:   "run [repository] [commit]",
	Short: "Run all builds defined by a repository's builds.yaml file. This command does not respect triggers defined in the builds.yaml file.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			log.Fatal("please provide a repository name and a commit sha")
		}

		gh, err := github.NewClient()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to setup GitHub client"))
		}

		ctx := context.Background()
		repository := args[0]
		commit := args[1]

		dir, err := gh.Clone(ctx, repository, commit)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "failed to clone GitHub repository github.com/%s", repository))
		}

		builds, err := configuration.Read(dir)
		if err != nil {
			log.Fatal(err)
		}
		if builds == nil {
			log.Fatalf("repository %s does not contain a builds.yaml file\n", repository)
		}

		id := configuration.BuildIdentification{
			Repository: repository,
			Commit:     commit,
			Directory:  dir,
		}

		builder, err := configuration.NewBuilder(ctx)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to setup builder"))
		}

		if err := builder.BuildAll(ctx, id, builds); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	buildCmd.AddCommand(buildRunCmd)
}
