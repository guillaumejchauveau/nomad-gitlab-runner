nomad {
  address = "http://127.0.0.1:4646"
  token_file = ""
  namespace = "default"
}

image = "ubuntu"
helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine-latest-x86_64-v15.10.0"

job {
  datacenters = ["dc1"]
  alloc_data_dir = "/alloc/data"

  upstreams {
    destination_name = "gitlab-http"
    local_bind_port = 50000
  }

  task "job" {
    driver = "docker"

    config = <<-EOT
      image = "{{.Image}}"
      {{if gt (len .Entrypoint) 0 -}}
      entrypoint = {{.Entrypoint | hcl}}
      {{else -}}
      command = "sh"
      {{end -}}
      args = ["{{.ExecScript}}"]
      {{with .Auth -}}
      auth = {
        username = "{{.Username}}"
        password = "{{.Password}}"
      }
      {{end -}}
      EOT
  }

  task "helper" {
    driver = "docker"

    config = <<-EOT
      image = "{{.Image}}"
      command = "sh"
      args = ["{{.ExecScript}}"]
      {{with .Auth -}}
      auth = {
        username = "{{.Username}}"
        password = "{{.Password}}"
      }
      {{end -}}
      EOT
  }

  task "service" {
    driver = "docker"

    config = <<-EOT
      image = "{{.Service.Name}}"
      entrypoint = {{.Service.Entrypoint | hcl}}
      {{if gt (len .Service.Command) 0 -}}
      command = "{{index .Service.Command 0}}"
      args = {{slice (.Service.Command | deref) 1 | hcl}}
      {{end -}}
      {{with .Auth -}}
      auth = {
        username = "{{.Username}}"
        password = "{{.Password}}"
      }
      {{end -}}
      EOT
  }
}
