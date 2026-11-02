// Command generate derives the isolated contract from unchanged standalone
// DTOs and an explicit operation whitelist. Run with go generate ./openapi.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

type object = map[string]any

func obj(value any) object         { return value.(map[string]any) }
func ref(kind, name string) object { return object{"$ref": "#/components/" + kind + "/" + name} }
func jsonResponse(description string, schema any) object {
	return object{"description": description, "content": object{"application/json": object{"schema": schema}}}
}
func textResponse(description string) object {
	return object{"description": description, "content": object{"text/plain": object{"schema": object{"type": "string"}}}}
}
func schema(properties object, required ...string) object {
	return object{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func integer() object { return object{"type": "integer", "minimum": 0} }
func constant(value any) object {
	kind := "string"
	if _, ok := value.(bool); ok {
		kind = "boolean"
	}
	return object{"type": kind, "const": value}
}
func response(schemaName string) object {
	return jsonResponse("Successful personal-mailbox response", ref("schemas", schemaName))
}
func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate() error {
	raw, err := os.ReadFile("openapi.json")
	if err != nil {
		return err
	}
	src := object{}
	if err = json.Unmarshal(raw, &src); err != nil {
		return err
	}
	oldPaths := obj(src["paths"])
	oldComponents := obj(src["components"])
	schemas := obj(oldComponents["schemas"])
	// All referenced DTOs are shared unless their runtime shape differs.
	delete(obj(schemas["BatchRequest"]), "additionalProperties")
	schemas["EmailStats"] = schema(object{"total": integer(), "unread": integer(), "read": integer(), "byDate": object{"type": "object", "additionalProperties": integer()}}, "total", "unread", "read", "byDate")
	obj(obj(schemas["Email"])["properties"])["source"] = object{"type": "string", "const": "", "description": "Filesystem source paths are redacted in multiuser mode."}
	obj(obj(schemas["Email"])["properties"])["envelope"] = ref("schemas", "Envelope")
	obj(schemas["Email"])["description"] = "Only the authenticated personal mailbox is visible. Ownership metadata is never returned. Envelope recipients are limited to this mailbox's matched SMTP recipients. Original MIME To/Cc/header/body content is not rewritten and may name other recipients. Reading does not mark the message read."
	schemas["MailboxMutation"] = schema(object{"success": integer(), "failed": object{"type": "integer", "const": 0}, "failedIDs": object{"type": "array", "maxItems": 0, "items": object{"type": "string"}}, "total": integer(), "count": integer(), "code": constant("MAILBOX_UPDATED")}, "success", "failed", "failedIDs", "total", "count", "code")
	capabilities := schema(object{"mcp": constant(false), "websocket": constant(false), "relay": constant(false), "webhooks": constant(false)}, "mcp", "websocket", "relay", "webhooks")
	schemas["MultiuserSettings"] = schema(object{"mode": constant("multiuser"), "capabilities": capabilities, "refreshIntervalSeconds": object{"type": "integer", "const": 15}}, "mode", "capabilities", "refreshIntervalSeconds")
	schemas["DisabledOutgoing"] = schema(object{"enabled": constant(false)}, "enabled")
	schemas["DisabledRelayJobs"] = schema(object{"jobs": object{"type": "array", "maxItems": 0, "items": object{}}, "more_possible": constant(false)}, "jobs", "more_possible")
	schemas["DisabledWebhookDeliveries"] = schema(object{"enabled": constant(false), "deliveries": object{"type": "array", "maxItems": 0, "items": object{}}, "more_possible": constant(false)}, "enabled", "deliveries", "more_possible")
	schemas["Liveness"] = schema(object{"status": constant("ok")}, "status")
	schemas["Readiness"] = schema(object{"status": object{"type": "string", "enum": []string{"ready", "unready"}}}, "status")
	headers := object{"Cache-Control": object{"description": "Sensitive responses are private and must not be stored.", "schema": object{"type": "string", "const": "private, no-store"}}, "X-OwlMail-Identity": object{"description": "Opaque stable assertion marker, only after trusted identity verification; clients clear old mailbox state when it changes. It is not an authentication credential.", "schema": object{"type": "string", "pattern": "^[a-f0-9]{64}$"}}}
	responses := object{
		"Unauthorized": jsonResponse("Missing, malformed, duplicate or untrusted upstream identity. User-supplied headers do not authenticate.", ref("schemas", "Error")),
		"Forbidden":    textResponse("Pre-authentication browser Origin policy refused the request; the policy applies independently of identity."),
		"BadRequest":   object{"description": "Invalid request, selector or parameters; malformed JSON and some validation paths return plain text.", "content": object{"application/json": object{"schema": ref("schemas", "Error")}, "text/plain": object{"schema": object{"type": "string"}}}},
		"NotFound":     jsonResponse("Email is absent or belongs to another mailbox; the responses are indistinguishable.", ref("schemas", "Error")),
		"Unavailable":  jsonResponse("Identity/control-store or mailbox recovery is temporarily unavailable. No authentication fallback.", ref("schemas", "Error")),
	}
	obj(responses["Unavailable"])["content"].(map[string]any)["text/plain"] = object{"schema": object{"type": "string"}}
	security := object{"trustedProxyIdentity": object{"type": "apiKey", "in": "header", "name": "X-Auth-User", "description": "Trusted reverse-proxy identity assertion, NOT a user API key. Authenticate at Stargate upstream; clients must not forge this header. The actual peer must be a configured trusted proxy, which strips client identity headers and injects the successful upstream result. Header presence alone never authenticates. Header names are deployment-configurable and the served contract reports the effective names.", "x-owlmail-requires-trusted-peer": true}}
	doc := object{"openapi": "3.1.0", "jsonSchemaDialect": src["jsonSchemaDialect"], "info": object{"title": "OwlMail multiuser HTTP API", "version": obj(src["info"])["version"], "summary": "Isolated personal-mailbox API; experimental public startup remains gated", "description": "Only this multiuser whitelist is exposed. Public CLI multiuser startup remains gated pending a real Stargate verified-email assertion and release evidence. A stable upstream subject automatically obtains its own personal mailbox; email alone never authenticates. Historical pending mail is claimed only using a trustworthy verified-email assertion under the explicit claim policy. Absent or false verification leaves pending mail inaccessible. Address ownership now does not prove historical ownership. No Basic Auth, anonymous mailbox access, personal API token, shared mailbox, global settings writes, reload, Relay sends, WebSocket or MCP. Source/header content is not rewritten. The server URL includes the effective base pathname. Public unversioned healthz/readyz aliases and static browser assets are outside this versioned contract."}, "servers": []any{object{"url": "/api/v1", "description": "Effective base-path-aware personal API prefix"}}, "security": []any{object{"trustedProxyIdentity": []any{}}}, "tags": []any{object{"name": "Emails"}, object{"name": "Settings"}, object{"name": "Operations"}, object{"name": "Contract"}}, "components": object{"schemas": schemas, "parameters": oldComponents["parameters"], "responses": responses, "securitySchemes": security}, "paths": object{}, "x-owlmail-mode": "multiuser", "x-owlmail-public-startup-gated": true,
		"x-owlmail-identity":              object{"subjectHeader": "X-Auth-User", "emailHeader": "X-Auth-Email", "emailVerifiedHeader": "X-Auth-Email-Verified", "verifiedValue": "true", "namespace": "Deployment-fixed; cannot be selected by the client", "conflict": "An already-bound address is never automatically transferred to another subject; the new subject only retains its own mailbox."},
		"x-owlmail-disabled-capabilities": []string{"mcp", "websocket", "relay-send", "webhook-send", "s3", "legacy-compatibility", "global-settings-write", "global-reload"},
		"x-owlmail-smtp-quotas":           object{"transport": "SMTP quotas are not HTTP 429 responses. Exhausted capacity or RCPT attempt quota refuses submission temporarily with 451; auth attempt exhaustion rejects AUTH. Multi-recipient DATA is accepted or refused as one group, never partly charged.", "attempts": "AUTH and authenticated RCPT attempts use durable fixed-minute budgets and a nondecreasing clock watermark. Restart/clock rollback cannot reset them; unknown AUTH names share one global key.", "usage": "Per-mailbox and global hard limits count committed messages and logical EML + metadata + decoded-attachment bytes. Deletion releases capacity; recovery derives the same usage. Global limits refuse new mail rather than evict another user's mail. Merge over quota is deferred without denying personal inbox access.", "defaults": object{"pendingMailboxes": 1000, "rcptAttemptsPerAccountMinute": 60, "authAttemptsGlobalMinute": 120, "messagesPerMailbox": 10000, "diskMiBPerMailbox": 1024, "messagesGlobal": 100000, "diskMiBGlobal": 10240}, "configuration": "Positive limits are deployment-configurable. These documented defaults are not a per-request remaining-budget API."}}
	paths := obj(doc["paths"])
	allowed := map[string][]string{
		"/openapi.json": {"get"}, "/openapi.yaml": {"get"}, "/health": {"get"}, "/ready": {"get"}, "/settings": {"get"}, "/settings/outgoing": {"get"}, "/relay-jobs": {"get"}, "/webhook-deliveries": {"get"},
		"/emails": {"get", "delete"}, "/emails/preview": {"get"}, "/emails/stats": {"get"}, "/emails/cursor": {"get"}, "/emails/wait": {"post"}, "/emails/export": {"get"}, "/emails/read": {"patch"}, "/emails/batch": {"delete"}, "/emails/batch/read": {"patch"}, "/emails/{id}": {"get", "delete"}, "/emails/{id}/html": {"get"}, "/emails/{id}/source": {"get"}, "/emails/{id}/raw": {"get"}, "/emails/{id}/attachments/{filename}": {"get"}, "/emails/{id}/check": {"post"}, "/emails/{id}/read": {"patch"}, "/test-runs/{runID}/emails": {"delete"},
	}
	for path, methods := range allowed {
		item := object{}
		for _, method := range methods {
			operation := obj(obj(oldPaths[path])[method])
			delete(operation, "security")
			delete(operation, "x-owlmail-public")
			operation["operationId"] = "multiuser" + strings.ToUpper(operation["operationId"].(string)[:1]) + operation["operationId"].(string)[1:]
			opResponses := obj(operation["responses"])
			for status, response := range opResponses {
				if inline, ok := response.(map[string]any); ok {
					delete(inline, "example")
					if content, ok := inline["content"].(map[string]any); ok {
						for _, media := range content {
							delete(obj(media), "example")
						}
					}
				}
				if status == "401" || status == "403" {
					continue
				}
			}
			opResponses["401"] = ref("responses", "Unauthorized")
			opResponses["403"] = ref("responses", "Forbidden")
			opResponses["503"] = ref("responses", "Unavailable")
			operation["description"] = "Only the authenticated personal mailbox is accessed. Other users' IDs are indistinguishable from absent IDs."
			item[method] = operation
		}
		paths[path] = item
	}
	// Personal mutations intentionally have a different response from legacy APIResponse/BatchResult.
	for _, endpoint := range []struct{ path, method string }{{"/emails", "delete"}, {"/emails/read", "patch"}, {"/emails/batch", "delete"}, {"/emails/batch/read", "patch"}, {"/emails/{id}", "delete"}, {"/emails/{id}/read", "patch"}} {
		operation := obj(obj(paths[endpoint.path])[endpoint.method])
		operation["summary"] = "Update only the current personal mailbox"
		operation["description"] = "The complete deduplicated ID set is ownership-checked before mutation. A foreign or absent ID rejects the whole batch before changes. A later storage failure may interrupt a valid batch and returns 409; this is not a transactional multi-message delete. Clearing affects only this mailbox."
		operation["responses"] = object{"200": response("MailboxMutation"), "400": ref("responses", "BadRequest"), "401": ref("responses", "Unauthorized"), "403": ref("responses", "Forbidden"), "404": ref("responses", "NotFound"), "409": jsonResponse("Owned mailbox update could not complete; inspect current state before retrying.", ref("schemas", "Error")), "503": ref("responses", "Unavailable")}
	}
	for path, name := range map[string]string{"/settings": "MultiuserSettings", "/settings/outgoing": "DisabledOutgoing", "/relay-jobs": "DisabledRelayJobs", "/webhook-deliveries": "DisabledWebhookDeliveries"} {
		operation := obj(obj(paths[path])["get"])
		operation["parameters"] = []any{}
		operation["tags"] = []string{"Settings"}
		operation["responses"] = object{"200": response(name), "401": ref("responses", "Unauthorized"), "403": ref("responses", "Forbidden"), "503": ref("responses", "Unavailable")}
		operation["description"] = "Read-only capability status. Outgoing delivery is disabled and no global settings, secrets or other users' delivery history are returned."
	}
	for _, path := range []string{"/health", "/ready"} {
		operation := obj(obj(paths[path])["get"])
		operation["security"] = []any{}
		operation["x-owlmail-public"] = true
		operation["description"] = "Public cached probe, subject to the independent browser Origin guard. No user, mailbox, credential or storage-path details."
		operation["responses"] = object{"200": response(map[string]string{"/health": "Liveness", "/ready": "Readiness"}[path]), "403": ref("responses", "Forbidden")}
		if path == "/ready" {
			obj(operation["responses"])["503"] = response("Readiness")
		}
	}
	stats := obj(obj(paths["/emails/stats"])["get"])
	stats["description"] = "Counts and byDate include only this personal mailbox; no global storage/index metrics are returned."
	cursor := obj(obj(paths["/emails/cursor"])["get"])
	cursor["summary"] = "Get an opaque personal-mailbox delivery cursor"
	cursor["description"] = "The encrypted cursor is bound to this mailbox and current process epoch. It reveals no global sequence, cannot be reused across users, and becomes invalid after restart. Obtain it before triggering a send."
	wait := obj(obj(paths["/emails/wait"])["post"])
	wait["description"] = "Long-poll only this mailbox after its opaque cursor. count means at least count retained matching deliveries. A 200 result can time out: check timed_out, not only HTTP status. Cursors from another mailbox or an earlier process fail with 400. Capacity is 8 waiters per mailbox and 64 globally; 429 has no Retry-After guarantee."
	props := obj(obj(obj(obj(wait["requestBody"])["content"])["application/json"])["schema"])
	wp := obj(props["properties"])
	obj(wp["count"])["minimum"] = 0
	obj(wp["count"])["description"] = "0 selects the default 1; effective range 1..100."
	obj(wp["timeout_seconds"])["minimum"] = 0
	obj(wp["timeout_seconds"])["description"] = "0 selects the default 30 seconds; effective range 1..120."
	obj(wait["responses"])["429"] = jsonResponse("Per-mailbox or global wait capacity reached. No Retry-After header is guaranteed.", ref("schemas", "Error"))
	export := obj(obj(paths["/emails/export"])["get"])
	export["description"] = "Export only owned messages, at most 1000 and 256 MiB total raw source. ids overrides selection filters after normal query validation. limit/offset do not truncate exports. Empty selection is 400; foreign IDs are 404; oversize is 413. Original EML is unmodified."
	er := obj(export["responses"])
	er["400"] = ref("responses", "BadRequest")
	er["404"] = ref("responses", "NotFound")
	er["413"] = textResponse("Export exceeds 1000 messages or 256 MiB raw source.")
	er["500"] = textResponse("Archive generation failed.")
	check := obj(obj(paths["/emails/{id}/check"])["post"])
	check["description"] = "Deterministic offline assertions on one owned email; assertion failure still returns 200 with passed=false. Does not mark read or fetch links. Request body above 65536 bytes returns 413."
	check["x-max-body-bytes"] = 65536
	obj(check["responses"])["400"] = ref("responses", "BadRequest")
	obj(check["responses"])["413"] = textResponse("Inspection request exceeds 65536 bytes.")
	run := obj(obj(paths["/test-runs/{runID}/emails"])["delete"])
	run["description"] = "Delete a snapshot of at most 1000 matching emails in this personal mailbox. run_id is a selector, never authorization. Repeat while more_possible is true. A storage failure may partially complete and returns 409."
	obj(run["responses"])["400"] = ref("responses", "BadRequest")
	obj(run["responses"])["409"] = textResponse("Run cleanup could not complete.")
	// All authenticated response metadata is scoped; public probes never claim to supply it.
	for path, item := range paths {
		if path == "/health" || path == "/ready" {
			continue
		}
		for _, rawOp := range obj(item) {
			for status, rawResponse := range obj(obj(rawOp)["responses"]) {
				if status == "403" {
					continue
				}
				if inline, ok := rawResponse.(map[string]any); ok {
					if _, isRef := inline["$ref"]; !isRef {
						inline["headers"] = headers
					}
				}
			}
		}
	}
	// Drop every component not reachable from the isolated paths or security.
	trimComponents(doc)
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err = os.WriteFile("multiuser.json", encoded, 0644); err != nil {
		return err
	}
	yamlEncoded, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile("multiuser.yaml", yamlEncoded, 0644)
}
func trimComponents(doc object) {
	components := obj(doc["components"])
	retained := object{"securitySchemes": components["securitySchemes"]}
	seen := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if raw, ok := typed["$ref"].(string); ok && strings.HasPrefix(raw, "#/components/") {
				parts := strings.Split(strings.TrimPrefix(raw, "#/components/"), "/")
				if len(parts) == 2 && !seen[raw] {
					seen[raw] = true
					kind, name := parts[0], parts[1]
					bucket, ok := retained[kind].(map[string]any)
					if !ok {
						bucket = object{}
						retained[kind] = bucket
					}
					target := obj(components[kind])[name]
					bucket[name] = target
					visit(target)
				}
			}
			for key, next := range typed {
				if key != "components" {
					visit(next)
				}
			}
		case []any:
			for _, next := range typed {
				visit(next)
			}
		}
	}
	visit(doc)
	doc["components"] = retained
}
