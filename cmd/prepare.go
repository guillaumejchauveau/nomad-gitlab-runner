package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"giruno/gitlab"
	"giruno/internals"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
)

var exec_script = `
if [ -x /usr/local/bin/bash ]; then
	echo "/usr/local/bin/bash"
elif [ -x /usr/bin/bash ]; then
	echo "/usr/bin/bash"
elif [ -x /bin/bash ]; then
	echo "/bin/bash"
elif [ -x /usr/local/bin/sh ]; then
	echo "/usr/local/bin/sh"
elif [ -x /usr/bin/sh ]; then
	echo "/usr/bin/sh"
elif [ -x /bin/sh ]; then
	echo "/bin/sh"
elif [ -x /busybox/sh ]; then
	echo "/busybox/sh"
else
	echo "Could not find compatible shell"
	exit 1
fi
mkfifo /tmp/stop_task
read _ < /tmp/stop_task
`

var prepareCmd = &cobra.Command{
	Use:          "prepare",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, ok := os.LookupEnv("JOB_ENV_ID")
		if !ok {
			return fmt.Errorf("no JOB_ENV_ID set")
		}

		// Extract job parameters from GitLab Runner-provided environment.

		image := os.Getenv("CUSTOM_ENV_CI_JOB_IMAGE")
		if image == "" {
			image = Config.DefaultImage
		}

		services := []gitlab.JobService{}
		env_services := os.Getenv("CUSTOM_ENV_CI_JOB_SERVICES")
		if env_services != "" {
			log.Println("With services")
			err := json.Unmarshal([]byte(env_services), &services)
			if err != nil {
				return err
			}
		}

		registry_auths := map[string]internals.RegistryAuth{}
		env_registry := os.Getenv("CUSTOM_ENV_CI_REGISTRY")
		if env_registry != "" {
			log.Println("With CI registry auth")
			user := os.Getenv("CUSTOM_ENV_CI_REGISTRY_USER")
			password := os.Getenv("CUSTOM_ENV_CI_REGISTRY_PASSWORD")
			if user == "" || password == "" {
				return fmt.Errorf("invalid registry auth")
			}
			registry_auths[env_registry] = internals.RegistryAuth{
				Username: user,
				Password: password,
			}
		}
		env_dependency_proxy := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_SERVER")
		if env_dependency_proxy != "" {
			log.Println("With Dependency Proxy auth")
			user := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_USER")
			password := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_PASSWORD")
			if user == "" || password == "" {
				return fmt.Errorf("invalid dependency proxy auth")
			}
			registry_auths[env_dependency_proxy] = internals.RegistryAuth{
				Username: user,
				Password: password,
			}
		}

		env_docker_auth_config := os.Getenv("CUSTOM_ENV_DOCKER_AUTH_CONFIG")
		if env_docker_auth_config != "" {
			log.Println("With Docker auth config")
			var docker_auth_config gitlab.DockerAuthConfig
			err := json.Unmarshal([]byte(env_docker_auth_config), &docker_auth_config)
			if err != nil {
				return err
			}
			for server, auth := range docker_auth_config.Auths {
				auth_decoded, err := base64.StdEncoding.DecodeString(auth)
				if err != nil {
					return err
				}
				username, password, found := strings.Cut(string(auth_decoded[:]), ":")
				if !found {
					return fmt.Errorf("invalid docker auth config")
				}
				registry_auths[server] = internals.RegistryAuth{
					Username: username,
					Password: password,
				}
			}
		}

		// Create Nomad job.
		// TODO: pull policy ? id_tokens ? secrets ?

		command_file_template := api.Template{
			EmbeddedTmpl: internals.Ptr(exec_script),
			DestPath:     internals.Ptr("local/exec_script.sh"),
			Perms:        internals.Ptr("755"),
		}

		job_task_type, err := Config.Job.GetTaskType("job")
		if err != nil {
			return err
		}
		job_task, err := job_task_type.CreateNomadTask(map[string]interface{}{
			"Image":      image,
			"ExecScript": "NOMAD_TASK_DIR/exec_script.sh",
			"Auth":       registry_auths[internals.DockerImageDomain(image)],
		})
		if err != nil {
			return err
		}
		job_task.Name = "job"
		job_task.Leader = true
		job_task.Templates = []*api.Template{
			&command_file_template,
		}

		helper_task_type, err := Config.Job.GetTaskType("helper")
		if err != nil {
			return err
		}
		helper_task, err := helper_task_type.CreateNomadTask(map[string]interface{}{
			"Image":      Config.HelperImage,
			"ExecScript": "NOMAD_TASK_DIR/exec_script.sh",
			"Auth":       registry_auths[internals.DockerImageDomain(Config.HelperImage)],
		})
		if err != nil {
			return err
		}
		helper_task.Name = "helper"
		helper_task.Templates = []*api.Template{
			&command_file_template,
		}

		job_spec := api.Job{
			ID:          &id,
			Type:        internals.Ptr("batch"),
			Datacenters: Config.Job.Datacenters,
			TaskGroups: []*api.TaskGroup{
				{
					Name: internals.Ptr("job"),
					RestartPolicy: &api.RestartPolicy{
						Attempts: internals.Ptr(0),
					},
					ReschedulePolicy: &api.ReschedulePolicy{
						Attempts:  internals.Ptr(0),
						Unlimited: internals.Ptr(false),
					},
					Tasks: []*api.Task{
						job_task,
						helper_task,
					},
				},
			},
		}

		// Add additionnal task for each CI service.
		service_task_type, err := Config.Job.GetTaskType("service")
		if err != nil {
			return err
		}
		for _, service := range services {
			task, err := service_task_type.CreateNomadTask(map[string]interface{}{
				"Service": service,
				"Auth":    registry_auths[internals.DockerImageDomain(service.Name)],
			})
			if err != nil {
				return err
			}
			task.Name = service.Name

			job_spec.TaskGroups[0].AddTask(task)
		}

		if len(Config.Job.Upstreams) > 0 {
			job_spec.TaskGroups[0].Networks = []*api.NetworkResource{
				{
					Mode: "bridge",
				},
			}

			job_spec.TaskGroups[0].Services = []*api.Service{
				{
					Connect: &api.ConsulConnect{
						SidecarService: &api.ConsulSidecarService{
							Proxy: &api.ConsulProxy{
								Upstreams: Config.Job.Upstreams,
							},
						},
					},
				},
			}
		}
		/*
			b, err := json.MarshalIndent(job_spec, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(b))*/

		// TODO: make cancellable https://docs.gitlab.com/runner/executors/custom.html#terminating-and-killing-executables

		log.Println("Creating client...")
		nomad, err := internals.NewNomad(Config)
		if err != nil {
			return err
		}

		log.Println("Validating job...")
		err = nomad.ValidateJob(&job_spec)
		if err != nil {
			return err
		}

		log.Println("Registering job...")
		err = nomad.RegisterJob(&job_spec)
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
		return nil
	},
}

func init() {
	rootCmd.AddCommand(prepareCmd)
}
