package internals

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/api"
	"github.com/spf13/viper"
)

type Nomad struct {
	client *api.Client
}

func NewNomadFromEnv() (*Nomad, error) {
	token := viper.GetString("nomad_token")
	if viper.IsSet("nomad_token_file") {
		secret_id_data, err := os.ReadFile(viper.GetString("nomad_token_file"))
		if err != nil {
			return nil, err
		}
		token = string(secret_id_data)
	}
	address := viper.GetString("nomad_address")

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
		Region:     viper.GetString("nomad_region"),
		SecretID:   token,
		Namespace:  viper.GetString("nomad_namespace"),
		HttpClient: http_client,
	})
	if err != nil {
		return nil, err
	}

	nomad := new(Nomad)
	nomad.client = client

	return nomad, nil
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
