package cmd

import (
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use: "nomad-gitlab-runner-executor",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.SetOutput(cmd.ErrOrStderr())
		viper.SetEnvPrefix("GIRUNO")
		viper.BindEnv("job_id")
		viper.BindEnv("datacenters")
		viper.SetDefault("datacenters", []string{"dc1"})
		viper.BindEnv("namespace")
		viper.BindEnv("driver")
		viper.SetDefault("driver", "docker")
		viper.BindEnv("connect")
		viper.SetDefault("connect", false)
		viper.BindEnv("helper_image")
		viper.SetDefault("helper_image", "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine-latest-x86_64-v15.10.0")

		viper.BindEnv("nomad_addr", "NOMAD_ADDR")

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
