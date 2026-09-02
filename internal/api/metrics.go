package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

var processStartedAt = time.Now()

type prometheusMetrics struct {
	startedAt time.Time
}

func newPrometheusMetrics() *prometheusMetrics {
	return &prometheusMetrics{startedAt: processStartedAt}
}

func (api *API) prometheusMetrics(c fiber.Ctx) error {
	stats := api.mailServer.GetMailboxMetrics()
	api.wsClientsLock.RLock()
	websocketClients := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	var output strings.Builder
	output.WriteString("# HELP owlmail_mailbox_messages Current messages stored in the mailbox.\n")
	output.WriteString("# TYPE owlmail_mailbox_messages gauge\n")
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"total\"} %d\n", stats.Total)
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"read\"} %d\n", stats.Read)
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"unread\"} %d\n", stats.Unread)
	output.WriteString("# HELP owlmail_emails_received_total Messages received since this process started.\n")
	output.WriteString("# TYPE owlmail_emails_received_total counter\n")
	fmt.Fprintf(&output, "owlmail_emails_received_total %d\n", stats.ReceivedMessages)
	output.WriteString("# HELP owlmail_emails_deleted_total Messages deleted since this process started.\n")
	output.WriteString("# TYPE owlmail_emails_deleted_total counter\n")
	fmt.Fprintf(&output, "owlmail_emails_deleted_total %d\n", stats.DeletedMessages)
	output.WriteString("# HELP owlmail_websocket_connections Current WebSocket clients.\n")
	output.WriteString("# TYPE owlmail_websocket_connections gauge\n")
	fmt.Fprintf(&output, "owlmail_websocket_connections %d\n", websocketClients)
	output.WriteString("# HELP owlmail_storage_cleanup_runs_total Mailbox retention cleanup runs.\n")
	output.WriteString("# TYPE owlmail_storage_cleanup_runs_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_cleanup_runs_total %d\n", stats.Storage.CleanupRuns)
	output.WriteString("# HELP owlmail_storage_deleted_messages_total Messages deleted by retention cleanup.\n")
	output.WriteString("# TYPE owlmail_storage_deleted_messages_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_deleted_messages_total %d\n", stats.Storage.DeletedMessages)
	output.WriteString("# HELP owlmail_storage_reclaimed_bytes_total Bytes reclaimed by retention cleanup.\n")
	output.WriteString("# TYPE owlmail_storage_reclaimed_bytes_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_reclaimed_bytes_total %d\n", stats.Storage.ReclaimedBytes)
	output.WriteString("# HELP owlmail_uptime_seconds Process uptime in seconds.\n")
	output.WriteString("# TYPE owlmail_uptime_seconds gauge\n")
	fmt.Fprintf(&output, "owlmail_uptime_seconds %.3f\n", time.Since(api.metrics.startedAt).Seconds())

	c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(http.StatusOK).SendString(output.String())
}
