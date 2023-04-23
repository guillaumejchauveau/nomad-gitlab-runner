package cmd

import (
	"fmt"
	"giruno/internals"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cleanupCmd = &cobra.Command{
	Use:          "cleanup",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !viper.IsSet("job_id") {
			return fmt.Errorf("no Nomad Job ID set")
		}
		id := viper.GetString("job_id")

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
	viper.MustBindEnv("job_id")
}
