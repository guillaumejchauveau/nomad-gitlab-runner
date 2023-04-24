package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"giruno/internals"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var find_shell_script = `
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
fi;`

var default_helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine-latest-x86_64-v15.10.0"

var datacenter string
var driver string
var upstreams []string
var mounts []string
var helper_image string
var default_image string

var prepareCmd = &cobra.Command{
	Use:          "prepare",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !viper.IsSet("job_id") {
			return fmt.Errorf("No Nomad job ID set")
		}
		id := viper.GetString("job_id")

		// Extract job parameters from GitLab Runner-provided environment.

		image := os.Getenv("CUSTOM_ENV_CI_JOB_IMAGE")
		if image == "" {
			image = default_image
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
		// TODO: pull policy ? id_tokens ? secrets ?

		job_spec := api.Job{
			ID:          &id,
			Type:        internals.Ptr("batch"),
			Datacenters: []string{datacenter},
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
						{
							Name:   "job",
							Driver: driver,
							Leader: true,
							Config: map[string]interface{}{
								"image":   image,
								// TODO "entrypoint": ???,
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
								"auth": auths[internals.DockerImageDomain(helper_image)].ToDriverConfig(),
							},
							Templates: []*api.Template{
								{
									EmbeddedTmpl: internals.Ptr(find_shell_script + "mkfifo /tmp/stop_task; read _ < /tmp/stop_task;"),
									DestPath:     internals.Ptr("local/command.sh"),
									Perms:        internals.Ptr("755"),
								},
							},
						},
					},
				},
			},
		}

		if len(upstreams) > 0 {
			job_spec.TaskGroups[0].Networks = []*api.NetworkResource{
				{
					Mode: "bridge",
				},
			}

			consul_upstreams := []*api.ConsulUpstream{}

			for _, upstream := range upstreams {
				parts := strings.Split(upstream, ":")
				if len(parts) != 2 {
					return fmt.Errorf("invalid upstream: %s", upstream)
				}
				local_bind_port, err := strconv.Atoi(parts[1])
				if err != nil {
					return fmt.Errorf("invalid upstream port: %s", upstream)
				}
				consul_upstreams = append(consul_upstreams, &api.ConsulUpstream{
					DestinationName: parts[0],
					LocalBindPort:   local_bind_port,
				})
			}

			job_spec.TaskGroups[0].Services = []*api.Service{
				{
					Connect: &api.ConsulConnect{
						SidecarService: &api.ConsulSidecarService{
							Proxy: &api.ConsulProxy{
								Upstreams: consul_upstreams,
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

		// TODO: make cancellable https://docs.gitlab.com/runner/executors/custom.html#terminating-and-killing-executables

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

	prepareCmd.PersistentFlags().StringVar(&datacenter, "datacenter", "dc1", "Task datacenter")
	prepareCmd.PersistentFlags().StringVar(&driver, "driver", "docker", "Task driver")
	prepareCmd.PersistentFlags().StringSliceVar(&upstreams, "upstream", []string{}, "Consul Connect upstream")
	prepareCmd.PersistentFlags().StringSliceVar(&mounts, "mount", []string{}, "Nomad Docker mount")
	prepareCmd.PersistentFlags().StringVar(&helper_image, "helper_image", default_helper_image, "Helper image")
	prepareCmd.PersistentFlags().StringVar(&default_image, "default_image", "ubuntu", "Default job image")
	viper.MustBindEnv("job_id")
}
