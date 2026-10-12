package outgoing

import "testing"

func TestRelayRuntimeReadinessTracksWorkersQueueAndClose(t *testing.T) {
	relay := NewOutgoingMail(&OutgoingConfig{Host: "localhost"})
	if status := relay.RuntimeReadiness(); !status.Ready() || status.Workers != 1 {
		t.Fatalf("started workers: %+v", status)
	}
	relay.Close()
	if status := relay.RuntimeReadiness(); status.Ready() || status.Workers != 0 {
		t.Fatalf("closed workers: %+v", status)
	}
	disabled := NewOutgoingMail(nil)
	defer disabled.Close()
	if status := disabled.RuntimeReadiness(); !status.Ready() || status.Enabled {
		t.Fatalf("disabled relay: %+v", status)
	}
	queue := &OutgoingMail{enabled: true, queue: make(chan *RelayTask, 2)}
	queue.liveWorkers.Store(1)
	queue.queue <- &RelayTask{}
	queue.queue <- &RelayTask{}
	if queue.RuntimeReadiness().Ready() {
		t.Fatal("full queue accepted")
	}
	<-queue.queue
	if !queue.RuntimeReadiness().Ready() {
		t.Fatal("queue capacity recovery missed")
	}
}
