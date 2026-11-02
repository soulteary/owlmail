package openapi

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

func TestMultiuserDocumentsEquivalentAndValid(t *testing.T) {
	rawJSON, err := documents.ReadFile("multiuser.json")
	if err != nil {
		t.Fatal(err)
	}
	rawYAML, err := documents.ReadFile("multiuser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var parsedJSON, parsedYAML any
	if err = yaml.Unmarshal(rawJSON, &parsedJSON); err != nil {
		t.Fatal(err)
	}
	if err = yaml.Unmarshal(rawYAML, &parsedYAML); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsedJSON, parsedYAML) {
		t.Fatal("static isolated JSON/YAML differ")
	}
	for _, raw := range [][]byte{rawJSON, rawYAML} {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromData(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err = doc.Validate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var doc map[string]any
	if err = json.Unmarshal(rawJSON, &doc); err != nil {
		t.Fatal(err)
	}
	validateDocumentShape(t, doc)
	validateLocalReferences(t, doc, doc)
	paths := doc["paths"].(map[string]any)
	for _, disabled := range []string{"/ws", "/mcp", "/emails/reload", "/emails/{id}/actions/relay", "/relay-jobs/{jobID}", "/relay-jobs/{jobID}/retry", "/webhook-deliveries/{jobID}/retry", "/version"} {
		if _, ok := paths[disabled]; ok {
			t.Errorf("disabled route documented: %s", disabled)
		}
	}
	settings := paths["/settings/outgoing"].(map[string]any)
	if len(settings) != 1 || settings["get"] == nil {
		t.Fatal("global settings write leaked into isolated contract")
	}
	security := doc["security"].([]any)
	if len(security) != 1 || security[0].(map[string]any)["trustedProxyIdentity"] == nil {
		t.Fatal("isolated identity security missing or anonymous alternative present")
	}
	for _, path := range []string{"/health", "/ready"} {
		if requirement, ok := paths[path].(map[string]any)["get"].(map[string]any)["security"].([]any); !ok || len(requirement) != 0 {
			t.Fatalf("%s not public", path)
		}
	}
	components := doc["components"].(map[string]any)
	schemes := components["securitySchemes"].(map[string]any)
	if len(schemes) != 1 || schemes["basicAuth"] != nil {
		t.Fatal("standalone credentials leaked into isolated contract")
	}
	proxy := schemes["trustedProxyIdentity"].(map[string]any)
	if proxy["x-owlmail-requires-trusted-peer"] != true || !strings.Contains(proxy["description"].(string), "NOT a user API key") {
		t.Fatal("assertion header described as sufficient authentication")
	}
	schemas := components["schemas"].(map[string]any)
	for _, old := range []string{"OutgoingSettings", "OutgoingConfig", "RelayJob", "WebSocketNew", "StorageStats"} {
		if schemas[old] != nil {
			t.Errorf("unused global component leaked: %s", old)
		}
	}
	stats := schemas["EmailStats"].(map[string]any)["properties"].(map[string]any)
	if stats["storage"] != nil {
		t.Fatal("stats promises global storage details")
	}
	cursor := paths["/emails/cursor"].(map[string]any)["get"].(map[string]any)["description"].(string)
	if !strings.Contains(cursor, "bound to this mailbox") || !strings.Contains(cursor, "restart") {
		t.Fatal("cursor scope/lifetime missing")
	}
	wait := paths["/emails/wait"].(map[string]any)["post"].(map[string]any)
	if !strings.Contains(wait["description"].(string), "8 waiters per mailbox and 64 globally") || !strings.Contains(wait["description"].(string), "timed_out") {
		t.Fatal("wait limits/timeout semantics missing")
	}
	if doc["x-owlmail-public-startup-gated"] != true || doc["x-owlmail-smtp-quotas"] == nil {
		t.Fatal("release gate or quota contract missing")
	}
}
func TestMultiuserServedContractUsesEffectiveHeadersAndBase(t *testing.T) {
	for _, prefix := range []string{"/api/v1", "/owlmail/api/v1", "/team%20mail/api/v1"} {
		jsonData, err := MultiuserJSONWithHeaders(prefix, "X-Test-Subject", "X-Test-Email", "X-Test-Verified")
		if err != nil {
			t.Fatal(err)
		}
		yamlData, err := MultiuserYAMLWithHeaders(prefix, "X-Test-Subject", "X-Test-Email", "X-Test-Verified")
		if err != nil {
			t.Fatal(err)
		}
		var jsonAsYAML, yamlAsYAML any
		if err = yaml.Unmarshal(jsonData, &jsonAsYAML); err != nil {
			t.Fatal(err)
		}
		if err = yaml.Unmarshal(yamlData, &yamlAsYAML); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(jsonAsYAML, yamlAsYAML) {
			t.Fatal("served JSON/YAML differ")
		}
		var doc map[string]any
		if err = json.Unmarshal(jsonData, &doc); err != nil {
			t.Fatal(err)
		}
		if doc["servers"].([]any)[0].(map[string]any)["url"] != prefix {
			t.Fatal("wrong server URL")
		}
		identity := doc["x-owlmail-identity"].(map[string]any)
		if identity["subjectHeader"] != "X-Test-Subject" || identity["emailHeader"] != "X-Test-Email" || identity["emailVerifiedHeader"] != "X-Test-Verified" {
			t.Fatal("stale default headers")
		}
		scheme := doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["trustedProxyIdentity"].(map[string]any)
		if scheme["name"] != "X-Test-Subject" {
			t.Fatal("security scheme did not follow configured header")
		}
	}
}
