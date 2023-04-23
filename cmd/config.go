package cmd

import (
	"encoding/json"
	"fmt"
	"giruno/internals"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:          "config",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := fmt.Sprintf("runner-%s-project-%s-concurrent-%s",
			os.Getenv("CUSTOM_ENV_CI_RUNNER_ID"),
			os.Getenv("CUSTOM_ENV_CI_PROJECT_ID"),
			os.Getenv("CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID"))

		settings := map[string]string{
			"GIRUNO_JOB_ID":    id,
			"NOMAD_ADDR":       viper.GetString("nomad_address"),
			"NOMAD_TOKEN":      viper.GetString("nomad_token"),
			"NOMAD_TOKEN_FILE": viper.GetString("nomad_token_file"),
			"NOMAD_REGION":     viper.GetString("nomad_region"),
			"NOMAD_NAMESPACE":  viper.GetString("nomad_namespace"),
		}

		config := internals.ConfigExecOutput{
			BuildsDir:         internals.Ptr(data_dir + "/builds/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")),
			CacheDir:          internals.Ptr(data_dir + "/cache/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")),
			BuildsDirIsShared: internals.Ptr(false),
			JobEnv:            &settings,
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(config)
	},
}

var data_dir string

func init() {
	rootCmd.AddCommand(configCmd)

	prepareCmd.PersistentFlags().StringVar(&data_dir, "data_dir", "", "Build data directory")
}
