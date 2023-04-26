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
      {{with .Service.Command -}}
      {{if gt (len .) 0 -}}
      command = "{{index . 0}}"
      args = {{slice . 1 | hcl}}
      {{end -}}
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
