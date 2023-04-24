nomad {
  address = "http://127.0.0.1:4646"
  token_file = "..."
  namespace = "default"
}

image = "ubuntu"
helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine-latest-x86_64-v15.10.0"

job {
  datacenters = ["dc1"]
  alloc_data_dir = "..."

  upstreams {
    destination_name = "gitlab-http"
    local_bind_port = 50000
  }

  task "job" {
    driver = "docker"

    config = <<EOT
      image = "{{.Image}}"
      command = "sh"
      args = ["{{.ExecScript}}"]
      auth = {
        username = "{{.Auth.Username}}"
        password = "{{.Auth.Password}}"
      }
    EOT
  }

  task "helper" {
    driver = "docker"

    config = <<EOT
      image = "{{.Image}}"
      command = "sh"
      args = ["{{.ExecScript}}"]
      auth = {
        username = "{{.Auth.Username}}"
        password = "{{.Auth.Password}}"
      }
    EOT
  }

  // var command *string
  // var args *[]string
  // if service.Command != nil {
  // 	service_command := *service.Command
  // 	if len(service_command) > 0 {
  // 		command = &service_command[0]
  // 		if len(service_command) > 1 {
  // 			tmp := service_command[1:]
  // 			args = &tmp
  // 		}
  // 	}
  // }
  task "service" {
    driver = "docker"

    config = <<EOT
      image = "{{.Service.Name}}"
      entrypoint = "{{.Service.Entrypoint}}"
      command = ""
      args = []
      auth = {
        username = "{{.Auth.Username}}"
        password = "{{.Auth.Password}}"
      }
    EOT
  }
}
