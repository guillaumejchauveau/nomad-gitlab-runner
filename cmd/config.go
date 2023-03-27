package cmd

import (
	"encoding/json"
	"nomad-gitlab-runner-executor/internals"
	"os"

	"github.com/spf13/cobra"
)

var nomadConfig = internals.NomadConfig{}

var configCmd = &cobra.Command{
	Use:          "config",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := internals.ConfigExecOutput{
			BuildsDir:         internals.Ptr("/builds/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")),
			CacheDir:          internals.Ptr("/cache/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")),
			BuildsDirIsShared: internals.Ptr(false),
			JobEnv:            &map[string]string{},
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(config)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
