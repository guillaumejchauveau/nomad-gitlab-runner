package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"nomad-gitlab-runner-executor/internals"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var find_shell_script = `
if [ -x /usr/local/bin/bash ]; then
	echo /usr/local/bin/bash
elif [ -x /usr/bin/bash ]; then
	echo /usr/bin/bash
elif [ -x /bin/bash ]; then
	echo /bin/bash
elif [ -x /usr/local/bin/sh ]; then
	echo /usr/local/bin/sh
elif [ -x /usr/bin/sh ]; then
	echo /usr/bin/sh
elif [ -x /bin/sh ]; then
	echo /bin/sh
elif [ -x /busybox/sh ]; then
	echo /busybox/sh
else
	exit 1
fi;`

var prepareCmd = &cobra.Command{
	Use:          "prepare",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := viper.GetString("job_id")
		datacenters := viper.GetStringSlice("datacenters")
		namespace := viper.GetString("namespace")

		driver := viper.GetString("driver")
		connect := viper.GetBool("connect")

		helper_image := viper.GetString("helper_image")

		// Extract job parameters from GitLab Runner-provided environment.

		image := os.Getenv("CUSTOM_ENV_CI_JOB_IMAGE")
		if image == "" {
			return fmt.Errorf("could not extract image from environment")
		}

		var services []internals.JobService
		env_services := os.Getenv("CUSTOM_ENV_CI_JOB_SERVICES")
		if env_services != "" {
			log.Println("With services")
			err := json.Unmarshal([]byte(env_services), &services)
			if err != nil {
				return err
			}
		} else {
			services = []internals.JobService{}
		}

		auths := map[string]*internals.DockerAuth{}
		env_registry := os.Getenv("CUSTOM_ENV_CI_REGISTRY")
		if env_registry != "" {
			log.Println("With CI registry auth")
			user := os.Getenv("CUSTOM_ENV_CI_REGISTRY_USER")
			password := os.Getenv("CUSTOM_ENV_CI_REGISTRY_PASSWORD")
			if user == "" || password == "" {
				return fmt.Errorf("invalid registry auth")
			}
			auths[env_registry] = &internals.DockerAuth{
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
			auths[env_dependency_proxy] = &internals.DockerAuth{
				Username: user,
				Password: password,
			}
		}

		env_docker_auth_config := os.Getenv("CUSTOM_ENV_DOCKER_AUTH_CONFIG")
		if env_docker_auth_config != "" {
			log.Println("With Docker auth config")
			var docker_auth_config internals.DockerAuthConfig
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
				auths[server] = &internals.DockerAuth{
					Username: username,
					Password: password,
				}
			}
		}

		// Create Nomad job.

		job_spec := api.Job{
			ID:          &id,
			Type:        internals.Ptr("batch"),
			Datacenters: datacenters,
			Namespace:   &namespace,
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
					Networks: []*api.NetworkResource{
						{
							Mode: "bridge",
						},
					},
					Tasks: []*api.Task{
						{
							Name:   "job",
							Driver: driver,
							Leader: true,
							Config: map[string]interface{}{
								"image":   image,
								"command": "sh",
								"args": []string{
									"${NOMAD_TASK_DIR}/command.sh",
								},
								"auth": auths[internals.DockerImageDomain(image)].ToDriverConfig(),
							},
							Templates: []*api.Template{
								{
									EmbeddedTmpl: internals.Ptr(find_shell_script + "mkfifo /tmp/stop_task; read _ < /tmp/stop_task;"),
									DestPath:     internals.Ptr("local/command.sh"),
									Perms:        internals.Ptr("755"),
								},
							},
						},
						{
							Name:   "helper",
							Driver: driver,
							Config: map[string]interface{}{
								"image":   helper_image,
								"command": "sh",
								"args": []string{
									"${NOMAD_TASK_DIR}/command.sh",
								},
								"auth":        auths[internals.DockerImageDomain(helper_image)].ToDriverConfig(),
								"interactive": true,
							},
							Templates: []*api.Template{
								{
									EmbeddedTmpl: internals.Ptr(find_shell_script + "read _;"),
									DestPath:     internals.Ptr("local/command.sh"),
									Perms:        internals.Ptr("755"),
								},
							},
						},
					},
				},
			},
		}

		// TODO: allow custom mounts
		// TODO: allow custom upstreams
		if connect {
			job_spec.TaskGroups[0].Services = []*api.Service{
				{
					Connect: &api.ConsulConnect{
						SidecarService: &api.ConsulSidecarService{
							Proxy: &api.ConsulProxy{
								Upstreams: []*api.ConsulUpstream{
									{
										DestinationName: "gitlab-http",
										LocalBindPort:   50000,
									},
									{
										DestinationName: "gitlab-registry",
										LocalBindPort:   50002,
									},
								},
							},
						},
					},
				},
			}
		}

		// Add additionnal task for each CI service.
		for _, service := range services {
			// Crappy code to convert the service command to the docker driver command
			var command *string
			var args *[]string
			if service.Command != nil {
				service_command := *service.Command
				if len(service_command) > 0 {
					command = &service_command[0]
					if len(service_command) > 1 {
						tmp := service_command[1:]
						args = &tmp
					}
				}
			}
			job_spec.TaskGroups[0].AddTask(&api.Task{
				Name:   service.Name,
				Driver: driver,
				Config: map[string]interface{}{
					"image":      service.Name,
					"entrypoint": service.Entrypoint,
					"command":    command,
					"args":       args,
					"auth":       auths[internals.DockerImageDomain(service.Name)].ToDriverConfig(),
				},
			})
		}

		log.Println("Creating client...")
		nomad, err := internals.NewNomadFromEnv()
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
