package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

type mcpRequestContextKey struct{}

func mcpContextTestServer(t *testing.T, parent context.Context, handler http.Handler) *httptest.Server {
	t.Helper()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.SetContext(parent)
		return c.Next()
	})
	app.All("/mcp", mcpHTTPHandler(handler))
	server := httptest.NewServer(adaptor.FiberApp(app))
	server.Client().Timeout = 3 * time.Second
	t.Cleanup(server.Close)
	return server
}

func TestMCPRequestContextPreservesParentAndCancelsOnCompletion(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	parent, cancel := context.WithDeadline(context.WithValue(context.Background(), mcpRequestContextKey{}, "agent"), deadline)
	defer cancel()
	captured := make(chan context.Context, 1)
	server := mcpContextTestServer(t, parent, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured <- request.Context()
		writer.WriteHeader(http.StatusNoContent)
	}))
	response, err := server.Client().Get(server.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	ctx := <-captured
	if ctx.Value(mcpRequestContextKey{}) != "agent" {
		t.Fatal("MCP request lost the configured parent context value")
	}
	if got, ok := ctx.Deadline(); !ok || !got.Equal(deadline) {
		t.Fatalf("MCP request deadline = %v, %t; want %v", got, ok, deadline)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("completed HTTP request context = %v; want canceled", ctx.Err())
	}
	if parent.Err() != nil {
		t.Fatal("completing an MCP request canceled its parent")
	}
}

func TestMCPRequestContextPropagatesParentCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			var parent context.Context
			var cancel context.CancelFunc
			cause := errors.New("agent request canceled")
			if deadline {
				parent, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				cause = context.DeadlineExceeded
			} else {
				ctx, stop := context.WithCancelCause(context.Background())
				parent, cancel = ctx, func() { stop(cause) }
			}
			defer cancel()
			started := make(chan struct{})
			result := make(chan error, 1)
			release := make(chan struct{})
			server := mcpContextTestServer(t, parent, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				close(started)
				select {
				case <-request.Context().Done():
					result <- context.Cause(request.Context())
					writer.WriteHeader(http.StatusNoContent)
				case <-release:
					writer.WriteHeader(http.StatusInternalServerError)
				}
			}))
			t.Cleanup(func() { close(release) })
			finished := make(chan error, 1)
			go func() {
				response, err := server.Client().Get(server.URL + "/mcp")
				if err == nil {
					_ = response.Body.Close()
				}
				finished <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("MCP request did not start")
			}
			if !deadline {
				cancel()
			}
			select {
			case got := <-result:
				if !errors.Is(got, cause) {
					t.Fatalf("request cancellation cause = %v; want %v", got, cause)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("MCP request did not receive parent cancellation")
			}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("canceled MCP request did not finish")
			}
		})
	}
}

func TestMCPStreamingContextLivesUntilHTTPHandlerReturns(t *testing.T) {
	captured := make(chan context.Context, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	server := mcpContextTestServer(t, context.Background(), http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured <- request.Context()
		_, _ = io.WriteString(writer, "start\n")
		writer.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(writer, "finish\n")
	}))
	t.Cleanup(releaseHandler)
	response, err := server.Client().Get(server.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "start\n" {
		t.Fatalf("initial stream data = %q; err=%v", first, err)
	}
	ctx := <-captured
	if ctx.Err() != nil {
		t.Fatalf("Flush prematurely canceled the streaming request: %v", ctx.Err())
	}
	releaseHandler()
	rest, err := io.ReadAll(reader)
	if err != nil || string(rest) != "finish\n" {
		t.Fatalf("final stream data = %q; err=%v", rest, err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("finished streaming request context = %v; want canceled", ctx.Err())
	}
}
