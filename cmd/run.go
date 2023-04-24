package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:          "run",
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, ok := os.LookupEnv("JOB_ENV_ID")
		if !ok {
			return fmt.Errorf("no JOB_ENV_ID set")
		}

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

		// TODO: make cancellable https://docs.gitlab.com/runner/executors/custom.html#terminating-and-killing-executables

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

		var shell string
		for {
			time.Sleep(time.Second)
			logs, err := nomad.GetTaskLogs(alloc, target, "stdout")
			if err != nil {
				return err
			}
			if logs != "" {
				shell = strings.Trim(logs, " \n\t\r")
				break
			}
		}
		log.Println("Using " + target + " shell " + shell)

		code, err := nomad.Exec(alloc, target, []string{
			shell,
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
}
