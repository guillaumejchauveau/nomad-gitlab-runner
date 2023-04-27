package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
mkdir -p /tmp/giruno
mkfifo /tmp/giruno/stop_task
read _ < /tmp/giruno/stop_task
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

		response_file_path := os.Getenv("JOB_RESPONSE_FILE")
		response_file_b, err := os.ReadFile(response_file_path)
		if err != nil {
			return fmt.Errorf("cannot read JOB_RESPONSE_FILE: %w", err)
		}
		response_file := map[string]json.RawMessage{}
		err = json.Unmarshal(response_file_b, &response_file)
		if err != nil {
			return fmt.Errorf("cannot unmarshal JOB_RESPONSE_FILE: %w", err)
		}

		// Extract job parameters from GitLab Runner-provided environment.

		image := os.Getenv("CUSTOM_ENV_CI_JOB_IMAGE")
		if image == "" {
			image = Config.DefaultImage
		}

		services := []gitlab.JobService{}
		env_services := os.Getenv("CUSTOM_ENV_CI_JOB_SERVICES")
		if env_services != "" {
			err := json.Unmarshal([]byte(env_services), &services)
			if err != nil {
				return err
			}
		}

		registry_auths := map[string]*internals.RegistryAuth{}
		env_registry := os.Getenv("CUSTOM_ENV_CI_REGISTRY")
		if env_registry != "" {
			user := os.Getenv("CUSTOM_ENV_CI_REGISTRY_USER")
			password := os.Getenv("CUSTOM_ENV_CI_REGISTRY_PASSWORD")
			if user == "" || password == "" {
				return fmt.Errorf("invalid registry auth")
			}
			registry_auths[env_registry] = &internals.RegistryAuth{
				Username: user,
				Password: password,
			}
		}
		env_dependency_proxy := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_SERVER")
		if env_dependency_proxy != "" {
			user := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_USER")
			password := os.Getenv("CUSTOM_ENV_CI_DEPENDENCY_PROXY_PASSWORD")
			if user == "" || password == "" {
				return fmt.Errorf("invalid dependency proxy auth")
			}
			registry_auths[env_dependency_proxy] = &internals.RegistryAuth{
				Username: user,
				Password: password,
			}
		}

		env_docker_auth_config := os.Getenv("CUSTOM_ENV_DOCKER_AUTH_CONFIG")
		if env_docker_auth_config != "" {
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
				registry_auths[server] = &internals.RegistryAuth{
					Username: username,
					Password: password,
				}
			}
		}

		// Create Nomad job specification.
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
		job_task_image_raw, ok := response_file["image"]
		if !ok {
			return fmt.Errorf("cannot extract image data from response file")
		}

		job_task_image := gitlab.JobResponseImage{}
		err = json.Unmarshal(job_task_image_raw, &job_task_image)
		if err != nil {
			return fmt.Errorf("cannot unmarshal image data from response file: %w", err)
		}
		job_task, err := job_task_type.CreateNomadTask(map[string]interface{}{
			"Image":      image,
			"Entrypoint": job_task_image.Entrypoint,
			"ExecScript": "${NOMAD_TASK_DIR}/exec_script.sh",
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
			"ExecScript": "${NOMAD_TASK_DIR}/exec_script.sh",
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

		// Add additionnal tasks for each CI service.
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
								Upstreams: Config.Job.ConsulUpstreams(),
							},
						},
					},
				},
			}
		}

		log.Println("Preparing environment")
		nomad, err := internals.NewNomad(Config)
		if err != nil {
			return err
		}

		c := make(chan os.Signal, 1)
		go func() {
			<-c
			log.Println("Received SIGTERM, exiting")
			nomad.Cancel()
		}()
		signal.Notify(c, syscall.SIGTERM)

		log.Println("Validating job")
		err = nomad.ValidateJob(&job_spec)
		if err != nil {
			return err
		}

		log.Println("Registering job")
		err = nomad.RegisterJob(&job_spec)
		if err != nil {
			return err
		}

		log.Println("Waiting for job allocation")
		_, dead, err := nomad.WaitForAllocation(id)
		if dead {
			return fmt.Errorf("allocation is dead")
		}
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(prepareCmd)
}
