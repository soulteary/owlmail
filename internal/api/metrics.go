package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

type prometheusMetrics struct {
	startedAt time.Time
	received  atomic.Uint64
	deleted   atomic.Uint64
}

func newPrometheusMetrics() *prometheusMetrics {
	return &prometheusMetrics{startedAt: time.Now()}
}

func metricUint(value interface{}) uint64 {
	switch typed := value.(type) {
	case uint64:
		return typed
	case uint:
		return uint64(typed)
	case int:
		if typed > 0 {
			return uint64(typed)
		}
	case int64:
		if typed > 0 {
			return uint64(typed)
		}
	case float64:
		if typed > 0 {
			return uint64(typed)
		}
	}
	return 0
}

func (api *API) prometheusMetrics(c fiber.Ctx) error {
	stats := api.mailServer.GetEmailStats()
	total := metricUint(stats["total"])
	read := metricUint(stats["read"])
	unread := metricUint(stats["unread"])

	storage, _ := stats["storage"].(map[string]interface{})
	api.wsClientsLock.RLock()
	websocketClients := len(api.wsClients)
	api.wsClientsLock.RUnlock()

	var output strings.Builder
	output.WriteString("# HELP owlmail_mailbox_messages Current messages stored in the mailbox.\n")
	output.WriteString("# TYPE owlmail_mailbox_messages gauge\n")
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"total\"} %d\n", total)
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"read\"} %d\n", read)
	fmt.Fprintf(&output, "owlmail_mailbox_messages{state=\"unread\"} %d\n", unread)
	output.WriteString("# HELP owlmail_emails_received_total Messages received since this process started.\n")
	output.WriteString("# TYPE owlmail_emails_received_total counter\n")
	fmt.Fprintf(&output, "owlmail_emails_received_total %d\n", api.metrics.received.Load())
	output.WriteString("# HELP owlmail_emails_deleted_total Messages deleted since this process started.\n")
	output.WriteString("# TYPE owlmail_emails_deleted_total counter\n")
	fmt.Fprintf(&output, "owlmail_emails_deleted_total %d\n", api.metrics.deleted.Load())
	output.WriteString("# HELP owlmail_websocket_connections Current WebSocket clients.\n")
	output.WriteString("# TYPE owlmail_websocket_connections gauge\n")
	fmt.Fprintf(&output, "owlmail_websocket_connections %d\n", websocketClients)
	output.WriteString("# HELP owlmail_storage_cleanup_runs_total Mailbox retention cleanup runs.\n")
	output.WriteString("# TYPE owlmail_storage_cleanup_runs_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_cleanup_runs_total %d\n", metricUint(storage["cleanupRuns"]))
	output.WriteString("# HELP owlmail_storage_deleted_messages_total Messages deleted by retention cleanup.\n")
	output.WriteString("# TYPE owlmail_storage_deleted_messages_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_deleted_messages_total %d\n", metricUint(storage["deletedMessages"]))
	output.WriteString("# HELP owlmail_storage_reclaimed_bytes_total Bytes reclaimed by retention cleanup.\n")
	output.WriteString("# TYPE owlmail_storage_reclaimed_bytes_total counter\n")
	fmt.Fprintf(&output, "owlmail_storage_reclaimed_bytes_total %d\n", metricUint(storage["reclaimedBytes"]))
	output.WriteString("# HELP owlmail_uptime_seconds Process uptime in seconds.\n")
	output.WriteString("# TYPE owlmail_uptime_seconds gauge\n")
	fmt.Fprintf(&output, "owlmail_uptime_seconds %.3f\n", time.Since(api.metrics.startedAt).Seconds())

	c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(http.StatusOK).SendString(output.String())
}
