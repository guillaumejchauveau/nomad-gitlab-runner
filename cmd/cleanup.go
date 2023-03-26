package cmd

import (
	"log"
	"nomad-gitlab-runner-executor/internals"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:          "cleanup",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := "test"

		log.Println("Creating client...")
		nomad, err := internals.NewNomadFromEnv()
		if err != nil {
			return err
		}

		log.Print("Waiting for job allocation... ")
		alloc, dead, err := nomad.WaitForAllocation(id)
		if err != nil {
			return err
		}
		log.Println(alloc.ID)

		if !dead {
			log.Println("Stopping allocation...")
			logs, err := nomad.GetTaskLogs(alloc, "job", "stdout")
			if err != nil {
				return err
			}
			log.Println("Using job shell " + logs)

			nomad.Exec(alloc, "job", []string{
				"sh",
				"-c",
				"echo > /tmp/stop_task",
			}, strings.NewReader(""), os.Stdout, os.Stderr)
		}

		log.Println("Deregistering job...")
		return nomad.DeregisterJob(id)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
