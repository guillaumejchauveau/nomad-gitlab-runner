package internals

type JobService struct {
	Name       string    `json:"name"`
	Alias      string    `json:"alias"`
	Entrypoint *[]string `json:"entrypoint"`
	Command    *[]string `json:"command"`
}

type DockerAuthConfig struct {
	Auths map[string]string `json:"auths"`
}

type DockerAuth struct {
	Username string
	Password string
}

func (s *DockerAuth) ToDriverConfig() map[string]string {
	if s == nil {
		return nil
	}
	return map[string]string{
		"username": s.Username,
		"password": s.Password,
	}
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
