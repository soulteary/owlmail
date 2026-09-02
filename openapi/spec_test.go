package openapi

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

func TestDocumentsParseAndStayEquivalent(t *testing.T) {
	jsonDocument, err := JSON("/api/v1")
	if err != nil {
		t.Fatal(err)
	}
	yamlDocument, err := YAML("/api/v1")
	if err != nil {
		t.Fatal(err)
	}

	var parsedJSON map[string]any
	if err := json.Unmarshal(jsonDocument, &parsedJSON); err != nil {
		t.Fatalf("parse JSON contract: %v", err)
	}
	var parsedJSONAsYAML any
	if err := yaml.Unmarshal(jsonDocument, &parsedJSONAsYAML); err != nil {
		t.Fatalf("parse JSON contract with YAML 1.2 parser: %v", err)
	}
	var parsedYAML any
	if err := yaml.Unmarshal(yamlDocument, &parsedYAML); err != nil {
		t.Fatalf("parse YAML contract: %v", err)
	}
	if !reflect.DeepEqual(parsedJSONAsYAML, parsedYAML) {
		t.Fatal("JSON and YAML contracts are not semantically equivalent")
	}
	if parsedJSON["openapi"] != "3.1.0" {
		t.Fatalf("openapi = %v, want 3.1.0", parsedJSON["openapi"])
	}
	if parsedJSON["jsonSchemaDialect"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected JSON Schema dialect: %v", parsedJSON["jsonSchemaDialect"])
	}

	loader := openapi3.NewLoader()
	for name, data := range map[string][]byte{"JSON": jsonDocument, "YAML": yamlDocument} {
		document, err := loader.LoadFromData(data)
		if err != nil {
			t.Fatalf("load %s contract with OpenAPI parser: %v", name, err)
		}
		if err := document.Validate(context.Background()); err != nil {
			t.Fatalf("validate %s contract as OpenAPI 3.1: %v", name, err)
		}
	}

	validateDocumentShape(t, parsedJSON)
	validateQuerySemantics(t, parsedJSON)
	validateLocalReferences(t, parsedJSON, parsedJSON)
}

func TestDocumentsUseEffectiveServerURL(t *testing.T) {
	for _, serverURL := range []string{"/api/v1", "/owlmail/api/v1", "/team%20mail/api/v1"} {
		jsonDocument, err := JSON(serverURL)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(jsonDocument, &document); err != nil {
			t.Fatal(err)
		}
		servers := document["servers"].([]any)
		if got := servers[0].(map[string]any)["url"]; got != serverURL {
			t.Fatalf("JSON server URL = %v, want %q", got, serverURL)
		}

		yamlDocument, err := YAML(serverURL)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(yamlDocument), "- url: "+strconv.Quote(serverURL)) {
			t.Fatalf("YAML contract does not contain server URL %q", serverURL)
		}
	}
}

func validateDocumentShape(t *testing.T, document map[string]any) {
	t.Helper()
	paths, ok := document["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatal("contract has no paths")
	}
	operationIDs := make(map[string]string)
	for path, rawPathItem := range paths {
		if !strings.HasPrefix(path, "/") {
			t.Errorf("path %q is not absolute within its server", path)
		}
		pathItem, ok := rawPathItem.(map[string]any)
		if !ok {
			t.Errorf("path item %q is not an object", path)
			continue
		}
		for method, rawOperation := range pathItem {
			if !isHTTPMethod(method) {
				continue
			}
			operation, ok := rawOperation.(map[string]any)
			if !ok {
				t.Errorf("%s %s operation is not an object", method, path)
				continue
			}
			operationID, ok := operation["operationId"].(string)
			if !ok || operationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
			} else if previous, exists := operationIDs[operationID]; exists {
				t.Errorf("operationId %q is shared by %s and %s %s", operationID, previous, method, path)
			} else {
				operationIDs[operationID] = method + " " + path
			}
			if responses, ok := operation["responses"].(map[string]any); !ok || len(responses) == 0 {
				t.Errorf("%s %s has no responses", method, path)
			}
		}
	}
}

func validateQuerySemantics(t *testing.T, document map[string]any) {
	t.Helper()
	components := document["components"].(map[string]any)
	parameters := components["parameters"].(map[string]any)
	wants := map[string]string{
		"DateTo": "Inclusive UTC-naive upper boundary at 00:00:00 of the following calendar date. Invalid values are ignored.",
		"SortBy": "Sort key. When omitted, results are sorted newest-first. Unsupported non-empty values preserve mailbox order.",
	}
	for name, want := range wants {
		parameter := parameters[name].(map[string]any)
		if got := parameter["description"]; got != want {
			t.Errorf("%s description = %q, want %q", name, got, want)
		}
	}
}

func validateLocalReferences(t *testing.T, root any, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
			if !localReferenceExists(root, strings.Split(strings.TrimPrefix(ref, "#/"), "/")) {
				t.Errorf("unresolved local reference %q", ref)
			}
		}
		for _, child := range typed {
			validateLocalReferences(t, root, child)
		}
	case []any:
		for _, child := range typed {
			validateLocalReferences(t, root, child)
		}
	}
}

func localReferenceExists(root any, parts []string) bool {
	current := root
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[part]
		if !ok {
			return false
		}
	}
	return true
}

func isHTTPMethod(value string) bool {
	switch value {
	case "get", "put", "post", "delete", "patch", "head", "options", "trace":
		return true
	default:
		return false
	}
}
