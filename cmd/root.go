package cmd

import (
	"giruno/internals"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use: "giruno",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.SetOutput(cmd.ErrOrStderr())
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		failure_code, env_err := strconv.Atoi(os.Getenv("SYSTEM_FAILURE_EXIT_CODE"))
		if _, ok := err.(*internals.BuildError); ok {
			failure_code, env_err = strconv.Atoi(os.Getenv("BUILD_FAILURE_EXIT_CODE"))
		}
		if env_err == nil {
			os.Exit(failure_code)
		}
		os.Exit(1)
	}
}

var nomad_addr string
var nomad_token string
var nomad_token_file string
var nomad_region string
var nomad_namespace string

func init() {
	viper.SetEnvPrefix("GIRUNO")
	configCmd.PersistentFlags().StringVar(&nomad_addr, "address", "http://127.0.0.1:4646", "Nomad address")
	viper.BindPFlag("nomad_address", configCmd.PersistentFlags().Lookup("address"))
	viper.MustBindEnv("nomad_address", "NOMAD_ADDR")

	configCmd.PersistentFlags().StringVar(&nomad_token, "token", "", "Nomad token")
	viper.BindPFlag("nomad_token", configCmd.PersistentFlags().Lookup("token"))
	viper.MustBindEnv("nomad_token", "NOMAD_TOKEN")

	configCmd.PersistentFlags().StringVar(&nomad_token_file, "token_file", "", "Nomad token file")
	viper.BindPFlag("nomad_token_file", configCmd.PersistentFlags().Lookup("token_file"))
	viper.MustBindEnv("nomad_token_file", "NOMAD_TOKEN_FILE")

	configCmd.PersistentFlags().StringVar(&nomad_region, "region", "", "Nomad region")
	viper.BindPFlag("nomad_region", configCmd.PersistentFlags().Lookup("region"))
	viper.MustBindEnv("nomad_region", "NOMAD_REGION")

	configCmd.PersistentFlags().StringVar(&nomad_namespace, "namespace", "default", "Nomad namespace")
	viper.BindPFlag("nomad_namespace", configCmd.PersistentFlags().Lookup("namespace"))
	viper.MustBindEnv("nomad_namespace", "NOMAD_NAMESPACE")
}
