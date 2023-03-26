package internals

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/hashicorp/nomad/api"
)

type Nomad struct {
	client *api.Client
}

func NewNomadFromEnv() (*Nomad, error) {
	nomad_address := "http://192.168.56.102:4646"
	nomad_secret_id := ""
	config := &api.Config{
		Address:  nomad_address,
		SecretID: nomad_secret_id,
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &Nomad{client: client}, nil
}

func (n *Nomad) ValidateJob(job *api.Job) error {
	res, _, err := n.client.Jobs().Validate(job, nil)
	if err != nil {
		return err
	}
	if res.Error != "" {
		return fmt.Errorf(res.Error)
	}
	return nil
}

func (n *Nomad) RegisterJob(job *api.Job) error {
	res, _, err := n.client.Jobs().Register(job, nil)
	if err != nil {
		return err
	}

	for {
		eval, _, err := n.client.Evaluations().Info(res.EvalID, nil)
		if err != nil {
			return err
		}
		if eval.Status == api.EvalStatusComplete {
			return nil
		}
		if eval.Status != "pending" {
			return fmt.Errorf(eval.Status)
		}
		time.Sleep(1 * time.Second)
	}
}

func (n *Nomad) WaitForAllocation(jobID string) (*api.Allocation, bool, error) {
	var id string
	for {
		allocs, _, err := n.client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return nil, true, err
		}
		if len(allocs) == 0 {
			return nil, true, fmt.Errorf("no allocations")
		}

		sort.Slice(allocs, func(i, j int) bool {
			return allocs[i].CreateIndex > allocs[j].CreateIndex
		})

		alloc_stub := allocs[0]
		id = alloc_stub.ID
		status := alloc_stub.ClientStatus

		if status == "complete" {
			break
		}

		if status == "pending" || len(alloc_stub.TaskStates) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if status != "running" {
			return nil, true, fmt.Errorf(status)
		}

		ready := true
		for _, task := range allocs[0].TaskStates {
			if task.State != "running" {
				ready = false
			}
		}
		if ready {
			break
		}
		time.Sleep(1 * time.Second)
	}
	alloc, _, err := n.client.Allocations().Info(id, nil)
	if err != nil {
		return nil, true, err
	}
	return alloc, alloc.ServerTerminalStatus(), nil
}

func (n *Nomad) GetTaskLogs(alloc *api.Allocation, task string, std string) (string, error) {
	reader, err := n.client.AllocFS().Cat(alloc, "alloc/logs/"+task+"."+std+".0", nil)
	if err != nil {
		return "", err
	}
	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	reader.Close()
	return string(logs), nil
}

func (n *Nomad) Exec(alloc *api.Allocation, task string, command []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return n.client.Allocations().Exec(context.TODO(), alloc, task, false, command, stdin, stdout, stderr, nil, nil)
}

func (n *Nomad) DeregisterJob(jobID string) error {
	_, _, err := n.client.Jobs().Deregister(jobID, false, nil)
	return err
}
