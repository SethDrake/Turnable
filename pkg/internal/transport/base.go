package transport

import (
	"io"
	"net"

	"github.com/theairblow/turnable/pkg/common"
)

// Handler represents a stream transport handler
type Handler interface {
	ID() string                                                 // Returns the unique ID of this handler
	WrapClient(conn net.PacketConn) (io.ReadWriteCloser, error) // Wraps a client packet connection
	WrapServer(conn net.PacketConn) (io.ReadWriteCloser, error) // Wraps a server packet connection
}

// Handlers represents transport handler registry.
var Handlers = common.NewRegistry[Handler]()

// init wires the transport registry and registers all built-in handlers.
func init() {
	common.TransportsHolder = Handlers
	Handlers.Register(&NoneHandler{})
	Handlers.Register(&SCTPHandler{})
	Handlers.Register(&KCPHandler{})
}

// GetHandler fetches a transport handler by its string ID.
func GetHandler(name string) (Handler, error) {
	return Handlers.Get(name)
}

// ListHandlers lists all transport handler string IDs.
func ListHandlers() []string {
	return Handlers.List()
}

// HandlerExists checks whether a transport handler with specified string ID exists.
func HandlerExists(name string) bool {
	return Handlers.Exists(name)
}
