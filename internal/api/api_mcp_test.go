package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/mcpserver"
)

func TestMCPRouteSharesBasicAuthAndBasePath(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	api := NewAPIWithHTTPS(mailbox, 1080, "localhost", "agent", "secret", true, "cert.pem", "key.pem")
	if err := api.SetMCPHandler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})); err != nil {
		t.Fatal(err)
	}
	if err := api.SetBasePathname("/owlmail"); err != nil {
		t.Fatal(err)
	}

	request, _ := http.NewRequest(http.MethodPost, "/owlmail/mcp", nil)
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP status = %d, want 401", response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodPost, "/owlmail/mcp", nil)
	request.SetBasicAuth("agent", "secret")
	response, err = api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("authenticated MCP status = %d, want 204", response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodPost, "/mcp", nil)
	request.SetBasicAuth("agent", "secret")
	response, err = api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unprefixed MCP status = %d, want 404", response.StatusCode)
	}

	if !api.httpsEnabled {
		t.Fatal("MCP route did not share the HTTPS-enabled API listener")
	}
}

func TestAuthenticatedMCPProtocolUsesPrefixedRoute(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()
	service, err := mcpserver.New(mailbox, mcpserver.Options{SessionTimeout: time.Minute, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()

	api := NewAPIWithAuth(mailbox, 1080, "localhost", "agent", "secret")
	if err := api.SetMCPHandler(service); err != nil {
		t.Fatal(err)
	}
	if err := api.SetBasePathname("/owlmail"); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(adaptor.FiberApp(api.app))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "auth-test", Version: "1"}, nil)
	if session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/owlmail/mcp", DisableStandaloneSSE: true,
	}, nil); err == nil {
		_ = session.Close()
		t.Fatal("MCP connection bypassed HTTP Basic Auth")
	}

	authenticatedClient := &http.Client{Transport: basicAuthTransport{
		username: "agent", password: "secret", next: http.DefaultTransport,
	}}
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/owlmail/mcp", HTTPClient: authenticatedClient,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	toolCount := 0
	for _, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		toolCount++
	}
	if toolCount != 7 {
		t.Fatalf("authenticated MCP tools = %d, want 7", toolCount)
	}
	resourceCount := 0
	for _, err := range session.Resources(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		resourceCount++
	}
	if resourceCount != 2 {
		t.Fatalf("authenticated MCP resources = %d, want 2", resourceCount)
	}
	promptCount := 0
	for _, err := range session.Prompts(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		promptCount++
	}
	if promptCount != 3 {
		t.Fatalf("authenticated MCP prompts = %d, want 3", promptCount)
	}
}

type basicAuthTransport struct {
	username string
	password string
	next     http.RoundTripper
}

func (transport basicAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.SetBasicAuth(transport.username, transport.password)
	return transport.next.RoundTrip(clone)
}

func TestMCPRouteIsDisabledByDefault(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()
	api := NewAPI(mailbox, 1080, "localhost")

	request, _ := http.NewRequest(http.MethodPost, "/mcp", nil)
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled MCP status = %d, want 404", response.StatusCode)
	}
}
