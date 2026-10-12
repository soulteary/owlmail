package mailserver

import (
	"context"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

type relayJobTestHealth struct{ status attachmentstore.HealthStatus }

func (provider *relayJobTestHealth) Snapshot() attachmentstore.HealthStatus { return provider.status }

func TestRelayJobReadinessSnapshotUsesEnabledOwnersFreshCache(t *testing.T) {
	server, err := NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	provider := &relayJobTestHealth{status: attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now()}}
	server.SetRelayJobReadinessProvider(provider)
	if snapshot := server.runtimeSnapshot(); snapshot.RelayJobs != nil {
		t.Fatal("disabled relay included durable dependency")
	}
	configuration, _ := startReadinessSMTP(t)
	if err := server.SetOutgoingConfig(configuration); err != nil {
		t.Fatal(err)
	}
	server.smtpReady.Store(true)
	server.probeMailStorage()
	server.probeOutgoingReadiness(context.Background())
	if snapshot := server.runtimeSnapshot(); snapshot.RelayJobs == nil || snapshot.Validate(server.host, server.port, time.Now()) != nil {
		t.Fatalf("ready owner not included: %+v", snapshot)
	}
	for _, age := range []time.Duration{time.Minute, -time.Minute} {
		provider.status.CheckedAt = time.Now().Add(-age)
		if status, enabled := server.GetRelayJobHealth(); !enabled || status.Ready() {
			t.Fatalf("invalid cache age accepted: %+v, enabled=%t", status, enabled)
		}
		if err := server.runtimeSnapshot().Validate(server.host, server.port, time.Now()); err == nil {
			t.Fatal("runtime snapshot concealed stale durable owner health")
		}
	}
	provider.status.CheckedAt = time.Now()
	_ = server.Close()
	if status, _ := server.GetRelayJobHealth(); status.State != attachmentstore.HealthClosed {
		t.Fatal("shutdown left owner ready")
	}
}

func TestRequiredDurableRelayOwnerStaysPendingBeforeRegistration(t *testing.T) {
	configuration, _ := startReadinessSMTP(t)
	server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{
		OutgoingConfig: configuration, RequireRelayJobReadiness: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	server.smtpReady.Store(true)
	server.probeMailStorage()
	server.probeOutgoingReadiness(context.Background())
	status := server.GetRuntimeReadiness()
	if status.RelayJobs == nil || status.RelayJobs.State != attachmentstore.HealthChecking {
		t.Fatalf("unregistered owner was omitted: %+v", status)
	}
	if err := status.ValidateService(time.Now()); err == nil {
		t.Fatal("configured service became ready before loading durable recovery")
	}
	provider := &relayJobTestHealth{status: attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now()}}
	server.SetRelayJobReadinessProvider(provider)
	if err := server.GetRuntimeReadiness().ValidateService(time.Now()); err != nil {
		t.Fatal(err)
	}
}
