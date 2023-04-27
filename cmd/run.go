package cmd

import (
	"fmt"
	"giruno/gitlab"
	"giruno/internals"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

		if strings.HasPrefix(stage, "step_") || strings.HasSuffix(stage, "_script") {
			target = "job"
		} else {
			target = "helper"
		}

		/*switch stage {
		case "get_sources", "restore_cache", "download_artifacts", "archive_cache", "archive_cache_on_failure", "upload_artifacts_on_success", "upload_artifacts_on_failure", "cleanup_file_variables":
			target = "helper"
		default:
			target = "job"
		}*/

		log.Printf("Running stage '%s'", stage)
		nomad, err := internals.NewNomad(Config)
		if err != nil {
			return err
		}

		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM)

		go func() {
			<-c
			log.Println("Received SIGTERM, exiting")
			nomad.Cancel()
		}()

		log.Println("Waiting for job allocation")
		alloc, dead, err := nomad.WaitForAllocation(id)
		if dead {
			return fmt.Errorf("allocation is dead")
		}
		if err != nil {
			return err
		}

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
			return gitlab.BuildError(code)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
