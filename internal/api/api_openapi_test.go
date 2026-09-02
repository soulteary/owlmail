package api

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestOpenAPIEndpointsUseBasePathAndContentTypes(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	if err := api.SetBasePathname("/owlmail/"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/owlmail/api/v1/openapi.json", "application/vnd.oai.openapi+json;version=3.1", `"url": "/owlmail/api/v1"`},
		{"/owlmail/api/v1/openapi.yaml", "application/vnd.oai.openapi;version=3.1", `- url: "/owlmail/api/v1"`},
	}
	for _, test := range tests {
		req, _ := http.NewRequest(http.MethodGet, test.path, nil)
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, body = %s", test.path, resp.StatusCode, body)
		}
		if got := resp.Header.Get("Content-Type"); got != test.contentType {
			t.Errorf("GET %s Content-Type = %q, want %q", test.path, got, test.contentType)
		}
		if !strings.Contains(string(body), test.contains) {
			t.Errorf("GET %s does not contain %q", test.path, test.contains)
		}
	}
}

func TestOpenAPIEndpointFollowsAuthenticationPolicy(t *testing.T) {
	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated OpenAPI status = %d, want 401", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	req.SetBasicAuth("user", "pass")
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated OpenAPI status = %d, want 200", resp.StatusCode)
	}
}

func TestOpenAPIContractCoversEveryVersionedRoute(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var contract struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contract); err != nil {
		t.Fatalf("parse served OpenAPI document: %v", err)
	}

	documented := make([]string, 0)
	for path, pathItem := range contract.Paths {
		for method := range pathItem {
			if isContractHTTPMethod(method) {
				documented = append(documented, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(documented)

	registered := make([]string, 0)
	for _, route := range api.app.GetRoutes(true) {
		if route.Method == http.MethodHead || !strings.HasPrefix(route.Path, "/api/v1") {
			continue
		}
		path := strings.TrimPrefix(route.Path, "/api/v1")
		registered = append(registered, route.Method+" "+openAPIPath(path))
	}
	sort.Strings(registered)

	if !reflect.DeepEqual(documented, registered) {
		t.Fatalf("OpenAPI route drift detected\ndocumented: %v\nregistered: %v", documented, registered)
	}
}

func openAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[index] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func isContractHTTPMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "patch", "options", "trace":
		return true
	default:
		return false
	}
}
