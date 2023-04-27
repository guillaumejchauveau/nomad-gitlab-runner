package internals

import (
	"context"
	"crypto/tls"
	"fmt"
	"giruno/config"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/api"
)

type RegistryAuth struct {
	Username string
	Password string
}

type Nomad struct {
	client *api.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewNomad(Config config.Config) (*Nomad, error) {
	address := Config.Nomad.Address

	http_client := cleanhttp.DefaultPooledClient()
	transport := http_client.Transport.(*http.Transport)
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 && parts[0] == "unix" {
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", parts[1])
		}
		address = "http://localhost"
	}

	if err := api.ConfigureTLS(http_client, api.DefaultConfig().TLSConfig); err != nil {
		return nil, err
	}
	client, err := api.NewClient(&api.Config{
		Address:    address,
		Region:     Config.Nomad.Region,
		SecretID:   Config.Nomad.Token,
		Namespace:  Config.Nomad.Namespace,
		HttpClient: http_client,
	})
	if err != nil {
		return nil, err
	}

	nomad := new(Nomad)
	nomad.client = client
	nomad.ctx, nomad.cancel = context.WithCancel(context.Background())

	return nomad, nil
}

func (n *Nomad) Cancel() {
	n.cancel()
}

func (n *Nomad) ValidateJob(job *api.Job) error {
	q := api.WriteOptions{}
	q.WithContext(n.ctx)
	res, _, err := n.client.Jobs().Validate(job, &q)
	if err != nil {
		return err
	}
	if res.Error != "" {
		return fmt.Errorf(res.Error)
	}
	return nil
}

func (n *Nomad) RegisterJob(job *api.Job) error {
	q_reg := api.WriteOptions{}
	q_reg.WithContext(n.ctx)
	res, _, err := n.client.Jobs().Register(job, &q_reg)
	if err != nil {
		return err
	}

	for {
		q_info := api.QueryOptions{}
		q_info.WithContext(n.ctx)
		eval, _, err := n.client.Evaluations().Info(res.EvalID, &q_info)
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
		q := api.QueryOptions{}
		q.WithContext(n.ctx)
		allocs, _, err := n.client.Jobs().Allocations(jobID, false, &q)
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
	q := api.QueryOptions{}
	q.WithContext(n.ctx)
	alloc, _, err := n.client.Allocations().Info(id, &q)
	if err != nil {
		return nil, true, err
	}
	return alloc, alloc.ServerTerminalStatus(), nil
}

func (n *Nomad) GetTaskLogs(alloc *api.Allocation, task string, std string) (string, error) {
	q := api.QueryOptions{}
	q.WithContext(n.ctx)
	reader, err := n.client.AllocFS().Cat(alloc, "alloc/logs/"+task+"."+std+".0", &q)
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
	return n.client.Allocations().Exec(n.ctx, alloc, task, false, command, stdin, stdout, stderr, nil, nil)
}

func (n *Nomad) DeregisterJob(jobID string) error {
	q := api.WriteOptions{}
	q.WithContext(n.ctx)
	_, _, err := n.client.Jobs().Deregister(jobID, false, &q) // TODO: purge
	return err
}
