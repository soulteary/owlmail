package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/owlmail/internal/relay"
)

// Install a fault-injected task owner at the API boundary. Production code
// shares the mailbox owner; these tests replace it only after stopping it.
func installWorkflowRelay(t *testing.T, api *API, options relay.Options, jobs ...relay.Job) *relay.Service {
	t.Helper()
	if api.relayService != nil {
		if err := api.relayService.Close(); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(api.mailServer.GetMailDir(), ".owlmail-meta", "relay-jobs")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		data, err := json.Marshal(job)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, job.ID+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := relay.NewWithOptions(api.mailServer, api.mailServer.GetMailDir(), options)
	if err := service.PersistenceError(); err != nil {
		t.Fatal(err)
	}
	api.relayService = service
	api.mailServer.SetRelayJobReadinessProvider(service)
	if err := api.mailServer.AddCloser(service); err != nil {
		t.Fatal(err)
	}
	return service
}
