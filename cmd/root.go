package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use: "giruno",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(cmd.ErrOrStderr())
		if cfgFile != "" {
			// Use config file from the flag.
			viper.SetConfigFile(cfgFile)
		} else {
			viper.AddConfigPath("/etc/giruno/")
			viper.SetConfigName("giruno")
		}

		err := viper.ReadInConfig()
		if err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				// Config file not found; ignore error if desired
			} else {
				return fmt.Errorf("fatal error config file: %w", err)
			}
		}
		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		// https://docs.gitlab.com/runner/executors/custom.html#error-handling
		failure_code, env_err := strconv.Atoi(os.Getenv("SYSTEM_FAILURE_EXIT_CODE"))
		if _, ok := err.(*internals.BuildError); ok {
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

var cfgFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file")
	viper.SetConfigType("hcl")
	viper.SetEnvPrefix("GIRUNO")

	viper.MustBindEnv("nomad.address", "NOMAD_ADDR")
	viper.MustBindEnv("nomad.token", "NOMAD_TOKEN")
	viper.MustBindEnv("nomad.token_file", "NOMAD_TOKEN_FILE")
	viper.MustBindEnv("nomad.region", "NOMAD_REGION")
	viper.MustBindEnv("nomad.namespace", "NOMAD_NAMESPACE")
}
