package cmd

import (
	"log"
	"os"
	"strconv"

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
		failure_code, err := strconv.Atoi(os.Getenv("SYSTEM_FAILURE_EXIT_CODE"))
		if err == nil {
			os.Exit(failure_code)
		}
		os.Exit(1)
	}
}

func init() {

}
