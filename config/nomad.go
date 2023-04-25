package config

import "github.com/hashicorp/nomad/api"

func (t *TaskType) CreateNomadTask(data map[string]interface{}) (*api.Task, error) {
	config, err := t.DriverConfig(data)
	if err != nil {
		return nil, err
	}

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
