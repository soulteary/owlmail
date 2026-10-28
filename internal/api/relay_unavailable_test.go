package api

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRelayEntrancesHandleUnavailableOwner(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	api.SetMailDevRESTCompat(true)
	api.relayService = nil
	for _, check := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPost, "/api/v1/emails/missing/actions/relay", 503},
		{http.MethodPost, "/email/missing/relay", 503},
		{http.MethodGet, "/api/v1/relay-jobs", 200},
		{http.MethodGet, "/api/v1/relay-jobs/missing", 404},
		{http.MethodPost, "/api/v1/relay-jobs/missing/retry", 404},
		{http.MethodPost, "/api/email/missing/relay", 500},
	} {
		t.Run(check.path, func(t *testing.T) {
			request, _ := http.NewRequest(check.method, check.path, nil)
			response, err := api.app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != check.status {
				t.Fatalf("status=%d want%d", response.StatusCode, check.status)
			}
		})
	}
}
