package api

import "github.com/soulteary/owlmail/internal/relay"

type RelayRecoveryInspection = relay.RelayRecoveryInspection

// InspectRelayRecovery inspects persisted tasks without changing or dispatching them.
func InspectRelayRecovery(directory string) (RelayRecoveryInspection, error) {
	return relay.InspectRecovery(directory)
}
