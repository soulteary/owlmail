package openapi

import (
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

//go:generate go run ./generate

// MultiuserJSON returns the isolated whitelist contract with default identity
// assertion header names. Servers use MultiuserJSONWithHeaders for their
// actual validated deployment configuration.
func MultiuserJSON(serverURL string) ([]byte, error) {
	return MultiuserJSONWithHeaders(serverURL, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
}
func MultiuserYAML(serverURL string) ([]byte, error) {
	return MultiuserYAMLWithHeaders(serverURL, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
}
func multiuserDocument(serverURL, subjectHeader, emailHeader, verifiedHeader string) (map[string]any, error) {
	raw, err := documents.ReadFile("multiuser.json")
	if err != nil {
		return nil, err
	}
	document := map[string]any{}
	if err = json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	servers, ok := document["servers"].([]any)
	if !ok || len(servers) != 1 {
		return nil, fmt.Errorf("invalid isolated contract servers")
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid isolated contract server")
	}
	server["url"] = serverURL
	components, ok := document["components"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid isolated contract components")
	}
	schemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid isolated security schemes")
	}
	proxy, ok := schemes["trustedProxyIdentity"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid isolated identity scheme")
	}
	proxy["name"] = subjectHeader
	identity, ok := document["x-owlmail-identity"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid isolated identity metadata")
	}
	identity["subjectHeader"] = subjectHeader
	identity["emailHeader"] = emailHeader
	identity["emailVerifiedHeader"] = verifiedHeader
	return document, nil
}

// MultiuserJSONWithHeaders documents effective, already validated assertion
// names. This function never configures or relaxes the identity trust boundary.
func MultiuserJSONWithHeaders(serverURL, subjectHeader, emailHeader, verifiedHeader string) ([]byte, error) {
	document, err := multiuserDocument(serverURL, subjectHeader, emailHeader, verifiedHeader)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(document, "", "  ")
}

// MultiuserYAMLWithHeaders renders the same semantic document as the JSON
// endpoint. YAML encoding quotes arbitrary header/path strings safely.
func MultiuserYAMLWithHeaders(serverURL, subjectHeader, emailHeader, verifiedHeader string) ([]byte, error) {
	document, err := multiuserDocument(serverURL, subjectHeader, emailHeader, verifiedHeader)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(document)
}
