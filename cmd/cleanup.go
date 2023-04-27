package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

		signal.Ignore(syscall.SIGTERM)

		log.Println("Cleaning up environment")
		nomad, err := internals.NewNomad(Config)
		if err != nil {
			return err
		}

		log.Println("Waiting for job allocation")
		alloc, dead, err := nomad.WaitForAllocation(id)
		if err != nil {
			return err
		}
		log.Println(alloc.ID)

		if !dead {
			log.Println("Stopping allocation")
			var shell string
			for {
				time.Sleep(time.Second)
				logs, err := nomad.GetTaskLogs(alloc, "job", "stdout")
				if err != nil {
					return err
				}
				if logs != "" {
					shell = strings.Trim(logs, " \n\t\r")
					break
				}
			}
			log.Println("Using job shell " + shell)
			nomad.Exec(alloc, "job", []string{
				shell,
			}, strings.NewReader("echo > /tmp/giruno/stop_task"), os.Stdout, os.Stderr)
		}

		log.Println("Deregistering job")
		return nomad.DeregisterJob(id)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
