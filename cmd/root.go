package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "nomad-gitlab-runner-executor",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.SetOutput(cmd.ErrOrStderr())
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

}
