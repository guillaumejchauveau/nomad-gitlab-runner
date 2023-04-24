package cmd

import (
	"encoding/json"
	"fmt"
	"giruno/internals"
	"os"
	"path"

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
			"JOB_ENV_ID": id,
		}

		data_dir := viper.GetString("job.alloc_data_dir")
		project_path := os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")
		config := internals.ConfigExecOutput{
			BuildsDir:         internals.Ptr(path.Join(data_dir, "builds", project_path)),
			CacheDir:          internals.Ptr(path.Join(data_dir, "cache", project_path)),
			BuildsDirIsShared: internals.Ptr(false),
			JobEnv:            &settings,
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(config)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
