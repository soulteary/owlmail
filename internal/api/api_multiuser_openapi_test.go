package api

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/identity"
	openapicontract "github.com/soulteary/owlmail/openapi"
	"go.yaml.in/yaml/v3"
)

func TestMultiuserContractCoversExactRegisteredWhitelist(t *testing.T) {
	f := newMultiFixture(t)
	response := f.request(t, "alice", "GET", "/api/v1/openapi.json", "")
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode, bodyText(t, response))
	}
	var contract struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.NewDecoder(response.Body).Decode(&contract); err != nil {
		t.Fatal(err)
	}
	documented := []string{}
	for path, item := range contract.Paths {
		for method := range item {
			if isContractHTTPMethod(method) {
				documented = append(documented, strings.ToUpper(method)+" "+path)
			}
		}
	}
	registered := []string{}
	for _, route := range f.api.app.GetRoutes(true) {
		if route.Method == http.MethodHead || !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		registered = append(registered, route.Method+" "+openAPIPath(strings.TrimPrefix(route.Path, "/api/v1")))
	}
	sort.Strings(documented)
	sort.Strings(registered)
	if !reflect.DeepEqual(documented, registered) {
		t.Fatalf("isolated contract drift\ndocumented %v\nregistered %v", documented, registered)
	}
	anonymous := f.request(t, "", "GET", "/api/v1/openapi.json", "")
	if anonymous.StatusCode != 401 {
		t.Fatal("contract endpoint bypassed trusted identity")
	}
	if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("X-OwlMail-Identity") == "" {
		t.Fatal("missing scoped response policy")
	}
}
func TestMultiuserOpenAPIEndpointsEffectivePrefixAndHeaders(t *testing.T) {
	f := newMultiFixture(t)
	proxy, err := identity.NewProxy("main", []string{"0.0.0.0/32"}, "X-Test-Subject", "X-Test-Email", "X-Test-Verified")
	if err != nil {
		t.Fatal(err)
	}
	f.api.multiuser.Proxy = proxy
	if err = f.api.SetBasePathname("/owlmail"); err != nil {
		t.Fatal(err)
	}
	var parsed [2]any
	for index, format := range []string{"json", "yaml"} {
		request, _ := http.NewRequest("GET", "/owlmail/api/v1/openapi."+format, nil)
		request.Header.Set("X-Test-Subject", "alice")
		response, err := f.api.app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 {
			t.Fatalf("%s: %d %s", format, response.StatusCode, data)
		}
		if !strings.Contains(response.Header.Get("Content-Type"), "version=3.1") {
			t.Fatal("missing OpenAPI content type")
		}
		if err = yaml.Unmarshal(data, &parsed[index]); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(parsed[0], parsed[1]) {
		t.Fatal("endpoint JSON/YAML differ")
	}
	doc := parsed[0].(map[string]any)
	if doc["servers"].([]any)[0].(map[string]any)["url"] != "/owlmail/api/v1" {
		t.Fatal("base path ignored")
	}
	headers := doc["x-owlmail-identity"].(map[string]any)
	if headers["subjectHeader"] != "X-Test-Subject" || headers["emailHeader"] != "X-Test-Email" || headers["emailVerifiedHeader"] != "X-Test-Verified" {
		t.Fatal("effective identity headers not documented")
	}
}

func TestMultiuserActualJSONResponsesMatchContract(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-id")
	f.mail(t, "bob", "bob-id")
	documentData, err := openapicontract.MultiuserJSON("/api/v1")
	if err != nil {
		t.Fatal(err)
	}
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromData(documentData)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ method, path, template, body string }{
		{"GET", "/api/v1/settings", "/settings", ""},
		{"GET", "/api/v1/settings/outgoing", "/settings/outgoing", ""},
		{"GET", "/api/v1/relay-jobs", "/relay-jobs", ""},
		{"GET", "/api/v1/webhook-deliveries", "/webhook-deliveries", ""},
		{"GET", "/api/v1/emails?limit=1&offset=0", "/emails", ""},
		{"GET", "/api/v1/emails/preview", "/emails/preview", ""},
		{"GET", "/api/v1/emails/stats", "/emails/stats", ""},
		{"GET", "/api/v1/emails/cursor", "/emails/cursor", ""},
		{"GET", "/api/v1/emails/alice-id", "/emails/{id}", ""},
		{"GET", "/api/v1/emails/bob-id", "/emails/{id}", ""},
		{"POST", "/api/v1/emails/alice-id/check", "/emails/{id}/check", `{"subject_contains":["shared"]}`},
		{"PATCH", "/api/v1/emails/batch/read", "/emails/batch/read", `{"ids":["alice-id"]}`},
	} {
		response := f.request(t, "alice", test.method, test.path, test.body)
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		operation := document.Paths.Value(test.template).GetOperation(test.method)
		status := strconv.Itoa(response.StatusCode)
		responseRef := operation.Responses.Value(status)
		if responseRef == nil {
			t.Fatalf("undocumented %s %s status %s: %s", test.method, test.path, status, data)
		}
		media := responseRef.Value.Content.Get("application/json")
		if media == nil || media.Schema == nil {
			t.Fatalf("missing JSON schema for %s %s %s", test.method, test.path, status)
		}
		var value any
		if err = json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		if err = media.Schema.Value.VisitJSON(value); err != nil {
			t.Fatalf("%s %s %s does not match contract: %v\n%s", test.method, test.path, status, err, data)
		}
	}
}
