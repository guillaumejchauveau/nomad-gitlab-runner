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
