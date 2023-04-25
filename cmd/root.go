package cmd

import (
	"giruno/config"
	"giruno/gitlab"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var cfgFile string

var Config config.Config

var rootCmd = &cobra.Command{
	Use: "giruno",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(cmd.ErrOrStderr())
		var err error
		Config, err = config.FromFile(cfgFile)
		if err != nil {
			return err
		}
		Config.WithEnv()
		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		// https://docs.gitlab.com/runner/executors/custom.html#error-handling
		failure_code, env_err := strconv.Atoi(os.Getenv("SYSTEM_FAILURE_EXIT_CODE"))
		if _, ok := err.(*gitlab.BuildError); ok {
			// Is the custom executor incompatible with allow_failure:exit_codes ?
			// https://docs.gitlab.com/ee/ci/yaml/#allow_failureexit_codes
			failure_code, env_err = strconv.Atoi(os.Getenv("BUILD_FAILURE_EXIT_CODE"))
		}
		if env_err == nil {
			os.Exit(failure_code)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file")
	rootCmd.MarkPersistentFlagRequired("config")
}
