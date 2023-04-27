package gitlab

import (
	"fmt"
)

type BuildError int

func (e BuildError) Error() string {
	return fmt.Sprintf("build error: %d", e)
}

type JobService struct {
	Name       string    `json:"name"`
	Alias      string    `json:"alias"`
	Entrypoint *[]string `json:"entrypoint"`
	Command    *[]string `json:"command"`
}

type DockerAuthConfig struct {
	Auths map[string]string `json:"auths"`
}

type JobResponseImage struct {
	Name         string           `json:"name"`
	Alias        string           `json:"alias,omitempty"`
	Command      []string         `json:"command,omitempty"`
	Entrypoint   []string         `json:"entrypoint,omitempty"`
	Ports        []map[string]any `json:"ports,omitempty"`
	Variables    []map[string]any `json:"variables,omitempty"`
	PullPolicies []string         `json:"pull_policy,omitempty"`
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
