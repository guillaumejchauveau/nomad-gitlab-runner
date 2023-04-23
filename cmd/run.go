package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func getTaskShell(nomad *internals.Nomad, alloc *api.Allocation, task string) (string, error) {
	for {
		time.Sleep(time.Second)
		logs, err := nomad.GetTaskLogs(alloc, task, "stdout")
		if err != nil {
			return "", err
		}
		if logs != "" {
			return logs, nil
		}
	}
}

var runCmd = &cobra.Command{
	Use:          "run",
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !viper.IsSet("job_id") {
			return fmt.Errorf("no Nomad Job ID set")
		}
		id := viper.GetString("job_id")

		script_data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		script := string(script_data)
		stage := args[1]

		var target string

		switch stage {
		case "get_sources", "restore_cache", "download_artifacts", "archive_cache", "archive_cache_on_failure", "upload_artifacts_on_success", "upload_artifacts_on_failure", "cleanup_file_variables":
			target = "helper"
		default:
			target = "job"
		}

		log.Println("Creating client...")
		nomad, err := internals.NewNomadFromEnv()
		if err != nil {
			return err
		}

		log.Print("Waiting for job allocation... ")
		alloc, dead, err := nomad.WaitForAllocation(id)
		if dead {
			return fmt.Errorf("allocation is dead")
		}
		if err != nil {
			return err
		}
		log.Println(alloc.ID)

		shell, err := getTaskShell(nomad, alloc, target)
		if err != nil {
			return err
		}
		log.Println("Using " + target + " shell " + shell)

		code, err := nomad.Exec(alloc, target, []string{
			strings.Trim(shell, " \n\t\r"),
		}, strings.NewReader(script), os.Stdout, os.Stderr)
		if err != nil {
			return err
		}
		if code != 0 {
			return internals.BuildError(code)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	viper.MustBindEnv("job_id")
}
