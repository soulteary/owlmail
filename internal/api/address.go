package api

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

// BoundAddress reports the actual HTTP(S) address between successful binding
// and listener shutdown. It is safe to read concurrently with startup.
func (api *API) BoundAddress() (mailserver.ListenerAddress, bool) {
	api.boundAddressMutex.RLock()
	defer api.boundAddressMutex.RUnlock()
	return api.boundAddress, api.boundAddress.Port != 0
}

// GetPort returns the actual bound HTTP(S) port, or the requested port before binding.
func (api *API) GetPort() int {
	if address, ok := api.BoundAddress(); ok {
		return address.Port
	}
	return api.port
}

func (api *API) publishBoundListener(listener net.Listener) error {
	tcp := listener.Addr().(*net.TCPAddr)
	api.boundAddressMutex.Lock()
	defer api.boundAddressMutex.Unlock()
	if api.listenerClosed {
		return fmt.Errorf("API server is closed")
	}
	api.boundListener = listener
	api.boundAddress = mailserver.ListenerAddress{Host: tcp.IP.String(), Port: tcp.Port}
	return nil
}

func (api *API) clearBoundAddress() {
	api.boundAddressMutex.Lock()
	api.boundListener = nil
	api.boundAddress = mailserver.ListenerAddress{}
	api.boundAddressMutex.Unlock()
}

// Close stops the HTTP listener, including a bound listener not yet passed to
// Fiber. Shutdown is bounded so a slow HTTP client cannot block process exit.
func (api *API) Close() error {
	api.boundAddressMutex.Lock()
	listener := api.boundListener
	api.listenerClosed = true
	api.boundAddressMutex.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	err := api.app.ShutdownWithTimeout(5 * time.Second)
	if errors.Is(err, net.ErrClosed) || errors.Is(err, fiber.ErrNotRunning) {
		return nil
	}
	return err
}
