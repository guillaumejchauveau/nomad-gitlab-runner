package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
)

var configCmd = &cobra.Command{
	Use:          "config",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		builds_dir := "/builds/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")
		cache_dir := "/cache/" + os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")
		builds_dir_is_shared := false

		config := api.ConfigExecOutput{
			BuildsDir:         &builds_dir,
			CacheDir:          &cache_dir,
			BuildsDirIsShared: &builds_dir_is_shared,
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(config)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
