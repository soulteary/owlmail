package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/emailcheck"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
)

type MultiuserOptions struct {
	Proxy       *identity.Proxy
	Store       *mailbox.Store
	ClaimPolicy mailbox.ClaimPolicy
}

const multiuserAssertionKey = "owlmail.multiuser.assertion"

// NewMultiuserAPIWithHTTPS constructs an isolated whitelist router. It never
// registers legacy/global routes, WebSocket listeners, or an MCP handler.
func NewMultiuserAPIWithHTTPS(server *mailserver.MailServer, port int, host string, httpsEnabled bool, certFile, keyFile string, options MultiuserOptions) (*API, error) {
	if server == nil || !server.RequiresOwnership() || options.Proxy == nil || options.Store == nil {
		return nil, errors.New("isolated server, identity proxy and control store required")
	}
	if options.ClaimPolicy != mailbox.VerifiedEmail && options.ClaimPolicy != mailbox.ExplicitHistory {
		return nil, errors.New("invalid claim policy")
	}
	api := &API{mailServer: server, port: port, host: host, httpsEnabled: httpsEnabled, httpsCertFile: certFile, httpsKeyFile: keyFile, multiuser: &options, metrics: newPrometheusMetrics()}
	api.setupRoutes()
	return api, nil
}
func (api *API) setupMultiuserRoutes() {
	app := fiber.New(fiber.Config{})
	app.Use(api.webOriginGuard())
	health := func(c fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) }
	app.Get("/healthz", health)
	app.Get(api.route("/healthz"), health)
	app.Get(api.route("/api/v1/health"), health)
	ready := func(c fiber.Ctx) error {
		status := "ready"
		code := 200
		if err := api.mailServer.GetRuntimeReadiness().ValidateService(time.Now()); err != nil {
			status = "unready"
			code = 503
		}
		return c.Status(code).JSON(fiber.Map{"status": status})
	}
	app.Get(api.route("/readyz"), ready)
	app.Get(api.route("/api/v1/ready"), ready)
	app.Use(func(c fiber.Ctx) error {
		c.Set("Cache-Control", "private, no-store")
		c.Set("Pragma", "no-cache")
		headers := http.Header{}
		for k, v := range c.Request().Header.All() {
			headers.Add(string(k), string(v))
		}
		assertion, err := api.multiuser.Proxy.Verify(c.RequestCtx().RemoteAddr().String(), headers)
		if err != nil {
			return c.Status(401).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Trusted identity required"))
		}
		identityMarker := sha256.Sum256([]byte("owlmail:identity:v1:" + assertion.Namespace + "\x00" + assertion.Subject))
		c.Set("X-OwlMail-Identity", fmt.Sprintf("%x", identityMarker))
		cacheKey := assertion.Namespace + "\x00" + assertion.Subject
		claimEvidence := fmt.Sprintf("%t:%s", assertion.EmailVerified, assertion.Email)
		prior, cached := api.multiuserClaims.Load(cacheKey)
		_, lookupErr := api.multiuser.Store.PersonalMailbox(c.Context(), assertion.Namespace, assertion.Subject)
		if !cached || prior != claimEvidence || lookupErr != nil {
			bound, bindErr := api.multiuser.Store.BindLogin(c.Context(), mailbox.Login{Namespace: assertion.Namespace, Subject: assertion.Subject, Email: assertion.Email, Verified: assertion.EmailVerified}, api.multiuser.ClaimPolicy)
			if bindErr != nil {
				return c.Status(503).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Identity service unavailable"))
			}
			if _, err := api.multiuser.Store.RunMergeBatch(c.Context(), bound.MailboxID, "", func(ctx context.Context, merge mailbox.Merge) (bool, error) {
				return api.mailServer.MergeOwnershipBatch(ctx, merge.SourceID, merge.TargetID, 32)
			}); err != nil {
				return c.Status(503).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Mailbox claim recovery unavailable"))
			}
			api.multiuserClaims.Store(cacheKey, claimEvidence)
		}

		// A route conflict does not expose or grant access to the previous owner's
		// mailbox. BindLogin still gives this identity its own personal mailbox.
		c.Locals(multiuserAssertionKey, assertion)
		return c.Next()
	})
	for name, contentType := range map[string]string{"app.js": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8", "favicon.svg": "image/svg+xml", "service-worker.js": "text/javascript; charset=utf-8", "help.js": "text/javascript; charset=utf-8", "help.css": "text/css; charset=utf-8"} {
		app.Get(api.route("/"+name), api.serveWebAsset(name, contentType))
	}
	app.Get(api.route("/"), api.serveWebAsset("index.html", "text/html; charset=utf-8"))
	app.Get(api.route("/help"), api.serveWebAsset("help.html", "text/html; charset=utf-8"))
	settings := func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"mode": "multiuser", "capabilities": fiber.Map{"mcp": false, "websocket": false, "relay": false, "webhooks": false}, "refreshIntervalSeconds": 15})
	}
	app.Get(api.route("/api/v1/openapi.json"), api.getOpenAPIJSON)
	app.Get(api.route("/api/v1/openapi.yaml"), api.getOpenAPIYAML)
	app.Get(api.route("/api/v1/settings"), settings)
	app.Get(api.route("/api/v1/me/mailbox"), api.multiMailboxStatus)
	app.Get(api.route("/api/v1/settings/outgoing"), func(c fiber.Ctx) error { return c.JSON(fiber.Map{"enabled": false}) })
	app.Get(api.route("/api/v1/relay-jobs"), func(c fiber.Ctx) error { return c.JSON(fiber.Map{"jobs": []any{}, "more_possible": false}) })
	app.Get(api.route("/api/v1/webhook-deliveries"), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"enabled": false, "deliveries": []any{}, "more_possible": false})
	})
	emails := app.Group(api.route("/api/v1/emails"))
	emails.Get("", api.multiList)
	emails.Get("/preview", api.multiList)
	emails.Get("/stats", func(c fiber.Ctx) error {
		return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error { return c.JSON(v.Stats()) })
	})
	emails.Get("/cursor", func(c fiber.Ctx) error {
		return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
			cursor, err := v.Cursor()
			if err != nil {
				return err
			}
			return c.JSON(fiber.Map{"cursor": cursor})
		})
	})
	emails.Post("/wait", api.multiWait)
	emails.Get("/export", api.multiExport)
	emails.Delete("", api.multiMutate)
	emails.Patch("/read", api.multiMutate)
	emails.Delete("/batch", api.multiMutate)
	emails.Patch("/batch/read", api.multiMutate)
	emails.Get("/:id", api.multiGet)
	emails.Get("/:id/html", api.multiGet)
	emails.Get("/:id/source", api.multiRaw)
	emails.Get("/:id/raw", api.multiRaw)
	emails.Get("/:id/attachments/:filename", api.multiAttachment)
	emails.Post("/:id/check", api.multiCheck)
	emails.Delete("/:id", api.multiMutate)
	emails.Patch("/:id/read", api.multiMutate)
	app.Delete(api.route("/api/v1/test-runs/:runID/emails"), api.multiDeleteRun)
	// Do not install SPA fallback: it must not disguise unsupported data exits.
	app.All("*", func(c fiber.Ctx) error { return c.SendStatus(404) })
	api.app = app
}
func (api *API) withMailbox(c fiber.Ctx, fn func(*mailserver.ScopedMailbox) error) error {
	assertion, ok := c.Locals(multiuserAssertionKey).(identity.Assertion)
	if !ok {
		return c.SendStatus(401)
	}
	return api.multiuser.Store.WithPersonalMailbox(c.Context(), assertion.Namespace, assertion.Subject, func(id string) error {
		view, err := api.mailServer.MailboxView(id)
		if err != nil {
			return c.SendStatus(503)
		}
		return fn(view)
	})
}
func (api *API) multiList(c fiber.Ctx) error {
	q, err := parseEmailQuery(c)
	if err != nil {
		return invalidEmailQuery(c, err)
	}
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		if strings.HasSuffix(c.Path(), "/preview") {
			items, total := v.Previews(q)
			return c.JSON(fiber.Map{"previews": items, "total": total, "limit": q.Limit, "offset": q.Offset})
		}
		items, total := v.List(q)
		return c.JSON(fiber.Map{"emails": items, "total": total, "limit": q.Limit, "offset": q.Offset})
	})
}
func multiMissing(c fiber.Ctx) error {
	return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
}
func (api *API) multiGet(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		e, err := v.Get(c.Params("id"))
		if err != nil {
			return multiMissing(c)
		}
		if strings.HasSuffix(c.Path(), "/html") {
			setEmailHTMLResponseHeaders(c)
			return c.SendString(e.HTML)
		}
		return c.JSON(e)
	})
}
func (api *API) multiRaw(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		r, size, err := v.OpenRaw(c.Params("id"))
		if err != nil {
			return multiMissing(c)
		}
		c.Set("Content-Type", "text/plain; charset=utf-8")
		if strings.HasSuffix(c.Path(), "/raw") {
			c.Set("Content-Type", "message/rfc822")
			c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeFilename(c.Params("id"))+".eml"))
		}
		// SendStream owns and closes the reader after the response is written.
		if size <= int64(^uint(0)>>1) {
			return c.SendStream(r, int(size))
		}
		return c.SendStream(r)
	})
}
func (api *API) multiAttachment(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		a, err := v.OpenAttachment(c.Params("id"), c.Params("filename"))
		if err != nil {
			return multiMissing(c)
		}
		setAttachmentResponseHeaders(c, a.ContentType, c.Params("filename"))
		if a.Size >= 0 && a.Size <= int64(^uint(0)>>1) {
			return c.SendStream(a.Body, int(a.Size))
		}
		return c.SendStream(a.Body)
	})
}
func (api *API) multiMutate(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		var ids []string
		if strings.Contains(c.Path(), "/batch") {
			var input struct {
				IDs []string `json:"ids"`
			}
			if err := c.Bind().Body(&input); err != nil {
				return c.SendStatus(400)
			}
			ids = input.IDs
			if len(ids) == 0 || len(ids) > maxBatchEmailIDs {
				return c.SendStatus(400)
			}
		} else if c.Params("id") != "" {
			ids = []string{c.Params("id")}
		} else {
			ids = v.IDs(mailserver.EmailQuery{})
		}
		// Validate all IDs before deleting or changing any item. Dedup keeps a
		// repeated ID from becoming a partial failure after the first deletion.
		unique := make([]string, 0, len(ids))
		seen := map[string]bool{}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				unique = append(unique, id)
			}
		}
		ids = unique
		if err := v.ValidateIDs(ids); err != nil {
			return multiMissing(c)
		}
		var n int
		var err error
		if c.Method() == http.MethodDelete {
			n, err = v.Delete(ids)
		} else {
			n, err = v.Read(ids)
		}
		if err != nil {
			return c.Status(409).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Mailbox update could not complete"))
		}
		return c.JSON(fiber.Map{"success": n, "failed": 0, "failedIDs": []string{}, "total": len(ids), "count": n, "code": "MAILBOX_UPDATED"})
	})
}
func (api *API) multiDeleteRun(c fiber.Ctx) error {
	run := c.Params("runID")
	if run == "" || mailserver.ValidateRunID(run) != nil {
		return c.SendStatus(400)
	}
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		emails, total := v.List(mailserver.EmailQuery{Limit: 1000, MatchStoreEmail: mailserver.DeliveryFilter{RunID: run}.Matches})
		ids := make([]string, 0, len(emails))
		for _, e := range emails {
			ids = append(ids, e.ID)
		}
		_, err := v.Delete(ids)
		if err != nil {
			return c.SendStatus(409)
		}
		return c.JSON(fiber.Map{"run_id": run, "deleted": ids, "failed": []string{}, "more_possible": total > len(ids)})
	})
}
func (api *API) multiCheck(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		e, err := v.Get(c.Params("id"))
		if err != nil {
			return multiMissing(c)
		}
		if len(c.Body()) > 64<<10 {
			return c.SendStatus(413)
		}
		var input emailcheck.Request
		if err = c.Bind().Body(&input); err != nil {
			return c.SendStatus(400)
		}
		report, err := emailcheck.Inspect(e, input)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid inspection request"))
		}
		return c.JSON(report)
	})
}
func (api *API) multiExport(c fiber.Ctx) error {
	return api.withMailbox(c, func(v *mailserver.ScopedMailbox) error {
		q, err := parseEmailQuery(c)
		if err != nil {
			return invalidEmailQuery(c, err)
		}
		q.Offset = 0
		q.Limit = maxExportMessages + 1
		var ids []string
		if value := c.Query("ids"); value != "" {
			ids = strings.Split(value, ",")
			if len(ids) > maxExportMessages {
				return c.SendStatus(413)
			}
			if err = v.ValidateIDs(ids); err != nil {
				return multiMissing(c)
			}
		} else {
			ids = v.IDs(q)
		}
		if len(ids) == 0 {
			return c.SendStatus(400)
		}
		if len(ids) > maxExportMessages {
			return c.SendStatus(413)
		}
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		var total int64
		for _, id := range ids {
			r, size, err := v.OpenRaw(id)
			if err != nil {
				_ = writer.Close()
				return multiMissing(c)
			}
			total += size
			if total > maxExportBytes {
				_ = r.Close()
				_ = writer.Close()
				return c.SendStatus(413)
			}
			out, err := writer.Create(sanitizeFilename(id) + ".eml")
			if err == nil {
				_, err = io.Copy(out, r)
			}
			_ = r.Close()
			if err != nil {
				_ = writer.Close()
				return c.SendStatus(500)
			}
		}
		if err = writer.Close(); err != nil {
			return err
		}
		c.Set("Content-Type", "application/zip")
		c.Set("Content-Disposition", "attachment; filename=emails.zip")
		return c.Send(buffer.Bytes())
	})
}
func (api *API) multiWait(c fiber.Ctx) error {
	var input struct {
		After string `json:"after"`
		mailserver.DeliveryFilter
		Count          int `json:"count"`
		TimeoutSeconds int `json:"timeout_seconds"`
	}
	if err := c.Bind().Body(&input); err != nil {
		return c.SendStatus(400)
	}
	if input.Count == 0 {
		input.Count = 1
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 30
	}
	if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 120 {
		return c.SendStatus(400)
	}
	var view *mailserver.ScopedMailbox
	// Do not hold the control-store mutation lock during a long poll. A personal
	// mailbox's owner never changes automatically; pending merge sources cannot
	// be selected by this authenticated lookup.
	if err := api.withMailbox(c, func(v *mailserver.ScopedMailbox) error { view = v; return nil }); err != nil {
		return err
	}
	if view == nil {
		return c.SendStatus(401)
	}
	result, err := view.Wait(c.Context(), input.After, input.DeliveryFilter, input.Count, time.Duration(input.TimeoutSeconds)*time.Second)
	if err != nil {
		status := 400
		if errors.Is(err, mailserver.ErrWaitCapacity) {
			status = 429
		}
		if errors.Is(err, mailserver.ErrMailboxClosed) {
			status = 503
		}
		return c.Status(status).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid or unavailable delivery wait"))
	}
	return c.JSON(result)
}
