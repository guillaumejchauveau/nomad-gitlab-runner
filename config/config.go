package config

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad/api"
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
	Datacenters  []string       `hcl:"datacenters"`
	AllocDataDir string         `hcl:"alloc_data_dir"`
	Upstreams    []*JobUpstream `hcl:"upstreams,block"`
	TaskTypes    []*TaskType    `hcl:"task,block"`
}

type JobUpstream struct {
	DestinationName      string                 `hcl:"destination_name,optional"`
	DestinationNamespace string                 `hcl:"destination_namespace,optional"`
	LocalBindPort        int                    `hcl:"local_bind_port,optional"`
	Datacenter           string                 `hcl:"datacenter,optional"`
	LocalBindAddress     string                 `hcl:"local_bind_address,optional"`
	MeshGateway          *api.ConsulMeshGateway `hcl:"mesh_gateway,block"`
	Config               map[string]any         `hcl:"config,optional"`
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

func (j *Job) ConsulUpstreams() []*api.ConsulUpstream {
	var upstreams []*api.ConsulUpstream
	for _, upstream := range j.Upstreams {
		upstreams = append(upstreams, &api.ConsulUpstream{
			DestinationName:      upstream.DestinationName,
			DestinationNamespace: upstream.DestinationNamespace,
			LocalBindPort:        upstream.LocalBindPort,
			Datacenter:           upstream.Datacenter,
			LocalBindAddress:     upstream.LocalBindAddress,
			MeshGateway:          upstream.MeshGateway,
			Config:               upstream.Config,
		})
	}
	return upstreams
}

func (t *TaskType) DriverConfig(task_data map[string]interface{}) (map[string]interface{}, error) {
	tmpl, err := template.
		New("driver_config").
		Funcs(template.FuncMap{
			"hcl": func(v interface{}) (string, error) {
				valTy, err := gocty.ImpliedType(v)
				if err != nil {
					return "", err
				}

				val, err := gocty.ToCtyValue(v, valTy)
				if err != nil {
					// This should never happen, since we should always be able
					// to decode into the implied type.
					panic(fmt.Sprintf("failed to encode %T as %#v: %s", v, valTy, err))
				}

				return string(hclwrite.TokensForValue(val).Bytes()), nil
			},
		}).
		Parse(t.ConfigTemplate)
	if err != nil {
		return nil, err
	}
	driver_config_hcl := new(bytes.Buffer)
	err = tmpl.Execute(driver_config_hcl, task_data)
	if err != nil {
		return nil, err
	}

	root, err := hcl.ParseBytes(driver_config_hcl.Bytes())
	if err != nil {
		return nil, err
	}
	driver_config_hcl.Reset()

	config := map[string]interface{}{}

	err = hcl.DecodeObject(&config, root)
	if err != nil {
		return nil, err
	}
	return config, nil
}
