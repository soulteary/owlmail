package api

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// mcpHTTPHandler gives asynchronous SDK work an owned context instead of the
// pooled fasthttp RequestCtx supplied by the generic HTTP adaptor.
func mcpHTTPHandler(handler http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		parent := c.Context()
		// Capture the channel while the RequestCtx belongs to this request;
		// asynchronous work must never call methods on the pooled context.
		serverDone := c.RequestCtx().Done()
		return adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ctx, cancel := context.WithCancel(parent)
			// Flush can return control to Fiber while the HTTP handler streams.
			// Cancel when the HTTP handler finishes, not when Fiber returns.
			defer cancel()
			if serverDone != nil {
				go func() {
					select {
					case <-serverDone:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			handler.ServeHTTP(writer, request.WithContext(ctx))
		}))(c)
	}
}
