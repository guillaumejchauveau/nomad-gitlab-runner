package cmd

import (
	"fmt"
	"log"
	"nomad-gitlab-runner-executor/internals"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:          "run",
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		id := "test"

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

		logs, err := nomad.GetTaskLogs(alloc, target, "stdout")
		if err != nil {
			return err
		}
		log.Println("Using " + target + " shell " + logs)

		code, err := nomad.Exec(alloc, target, []string{
			"sh",
			"-c",
			script,
		}, strings.NewReader(""), os.Stdout, os.Stderr)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("command exited with code %v", code)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
