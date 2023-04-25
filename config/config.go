package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

type Config struct {
	Nomad        Nomad  `hcl:"nomad,block"`
	DefaultImage string `hcl:"image"`
	HelperImage  string `hcl:"helper_image"`
	Job          Job    `hcl:"job,block"`
}

type Nomad struct {
	Address   string `hcl:"address"`
	Token     string `hcl:"token,optional"`
	TokenFile string `hcl:"token_file,optional"`
	Region    string `hcl:"region,optional"`
	Namespace string `hcl:"namespace"`
}

type Job struct {
	Datacenters  []string              `hcl:"datacenters"`
	AllocDataDir string                `hcl:"alloc_data_dir"`
	Upstreams    []*api.ConsulUpstream `hcl:"upstreams,block"`
	TaskTypes    []*TaskType           `hcl:"task,block"`
}

type TaskType struct {
	Type           string            `hcl:"type,label"`
	Driver         string            `hcl:"driver"`
	User           string            `hcl:"user,optional"`
	ConfigTemplate string            `hcl:"config"`
	Constraints    []*api.Constraint `hcl:"constraint,block"`
	Affinities     []*api.Affinity   `hcl:"affinity,block"`
	Resources      *api.Resources    `hcl:"resources,block"`
	Meta           map[string]string `hcl:"meta,optional"`
}

func FromFile(path string) (Config, error) {
	var config Config
	err := hclsimple.DecodeFile(path, nil, &config)
	if err != nil {
		return config, err
	}

	if config.Nomad.TokenFile != "" {
		token, err := os.ReadFile(config.Nomad.TokenFile)
		if err != nil {
			return config, err
		}
		config.Nomad.Token = string(token)
	}
	return config, nil
}

func (c *Config) WithEnv() {
	if v, ok := os.LookupEnv("NOMAD_ADDR"); ok {
		c.Nomad.Address = v
	}
	if v, ok := os.LookupEnv("NOMAD_TOKEN"); ok {
		c.Nomad.Token = v
	}
	if v, ok := os.LookupEnv("NOMAD_TOKEN_FILE"); ok {
		c.Nomad.TokenFile = v
	}
	if v, ok := os.LookupEnv("NOMAD_REGION"); ok {
		c.Nomad.Region = v
	}
	if v, ok := os.LookupEnv("NOMAD_NAMESPACE"); ok {
		c.Nomad.Namespace = v
	}
}

func (j *Job) GetTaskType(task_type string) (*TaskType, error) {
	for _, t := range j.TaskTypes {
		if t.Type == task_type {
			return t, nil
		}
	}
	return nil, fmt.Errorf("task type '%s' not found", task_type)
}

func (t *TaskType) DriverConfig(task_data map[string]interface{}) (map[string]interface{}, error) {
	tmpl, err := template.New("driver_config").Parse(t.ConfigTemplate)
	if err != nil {
		return nil, err
	}
	driver_config_hcl := new(bytes.Buffer)
	err = tmpl.Execute(driver_config_hcl, task_data)
	if err != nil {
		return nil, err
	}
	raw := map[string]cty.Value{}
	err = hclsimple.Decode("template.hcl", driver_config_hcl.Bytes(), nil, &raw)
	if err != nil {
		return nil, err
	}
	config := map[string]interface{}{}
	err = gocty.FromCtyValue(raw["args"], &config)
	if err != nil {
		panic(err)
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}

	fmt.Println(string(b))
	return config, nil
}
