package mailserver

import "net"

// ListenerAddress is an actual bound TCP listener address, with its OS-selected port.
type ListenerAddress struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// BoundAddress reports the actual SMTP address after all SMTP listeners bind.
// It is unavailable before binding and after the listener stops.
func (ms *MailServer) BoundAddress() (ListenerAddress, bool) {
	ms.boundAddressMutex.RLock()
	defer ms.boundAddressMutex.RUnlock()
	return ms.boundAddress, ms.boundAddress.Port != 0
}

func (ms *MailServer) publishBoundAddress(address net.Addr) {
	tcp := address.(*net.TCPAddr)
	ms.boundAddressMutex.Lock()
	ms.boundAddress = ListenerAddress{Host: tcp.IP.String(), Port: tcp.Port}
	ms.boundAddressMutex.Unlock()
}

func (ms *MailServer) clearBoundAddress() {
	ms.boundAddressMutex.Lock()
	ms.boundAddress = ListenerAddress{}
	ms.boundAddressMutex.Unlock()
}
