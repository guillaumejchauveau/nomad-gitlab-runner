package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:          "cleanup",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, ok := os.LookupEnv("JOB_ENV_ID")
		if !ok {
			return fmt.Errorf("no JOB_ENV_ID set")
		}

		// TODO: make cancellable https://docs.gitlab.com/runner/executors/custom.html#terminating-and-killing-executables

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
