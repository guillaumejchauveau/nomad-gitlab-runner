package cmd

import (
	"encoding/json"
	"fmt"
	"giruno/gitlab"
	"giruno/internals"
	"os"
	"path"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:          "config",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := fmt.Sprintf("runner-%s-project-%s-job-%s",
			os.Getenv("CUSTOM_ENV_CI_RUNNER_ID"),
			os.Getenv("CUSTOM_ENV_CI_PROJECT_ID"),
			os.Getenv("CUSTOM_ENV_CI_JOB_ID"))

		settings := map[string]string{
			"JOB_ENV_ID": id,
		}

		project_path := os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH")
		config := gitlab.ConfigExecOutput{
			BuildsDir:         internals.Ptr(path.Join(Config.Job.AllocDataDir, "builds", project_path)),
			CacheDir:          internals.Ptr(path.Join(Config.Job.AllocDataDir, "cache", project_path)),
			BuildsDirIsShared: internals.Ptr(false),
			JobEnv:            &settings,
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(config)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
