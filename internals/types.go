package internals

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/api"
)

type BuildError int

func (e BuildError) Error() string {
	return fmt.Sprintf("build error: %d", e)
}

type GitLabJobService struct {
	Name       string    `json:"name"`
	Alias      string    `json:"alias"`
	Entrypoint *[]string `json:"entrypoint"`
	Command    *[]string `json:"command"`
}

type GitLabDockerAuthConfig struct {
	Auths map[string]string `json:"auths"`
}

type RegistryAuth struct {
	Username string
	Password string
}

type ConfigTask struct {
	Driver      string
	User        string
	Config      string
	Constraints []*api.Constraint
	Affinities  []*api.Affinity
	Resources   *api.Resources
	Meta        map[string]string
}

func (t *ConfigTask) ToNomadTask(driver_config_data map[string]interface{}) (*api.Task, error) {
	driver_config_tmpl, err := template.New("driver_config").Parse(t.Config)
	if err != nil {
		return nil, err
	}
	driver_config_hcl := new(bytes.Buffer)
	err = driver_config_tmpl.Execute(driver_config_hcl, driver_config_data)
	if err != nil {
		return nil, err
	}
	config := map[string]interface{}{}
	hcl.Unmarshal(driver_config_hcl.Bytes(), &config)

	return &api.Task{
		Driver:      t.Driver,
		User:        t.User,
		Config:      config,
		Constraints: t.Constraints,
		Affinities:  t.Affinities,
		Resources:   t.Resources,
		Meta:        t.Meta,
	}, nil
}

// https://gitlab.com/gitlab-org/gitlab-runner/blob/main/executors/custom/api/config.go
// ConfigExecOutput defines the output structure of the config_exec call.
//
// This should be used to pass the configuration values from Custom Executor
// driver to the Runner.
type ConfigExecOutput struct {
	Driver *DriverInfo `json:"driver,omitempty"`

	Hostname  *string `json:"hostname,omitempty"`
	BuildsDir *string `json:"builds_dir,omitempty"`
	CacheDir  *string `json:"cache_dir,omitempty"`

	BuildsDirIsShared *bool `json:"builds_dir_is_shared,omitempty"`

	JobEnv *map[string]string `json:"job_env,omitempty"`

	Shell *string `json:"shell,omitempty"`
}

// DriverInfo wraps the information about Custom Executor driver details
// like the name or version
type DriverInfo struct {
	Name    *string `json:"name,omitempty"`
	Version *string `json:"version,omitempty"`
}
