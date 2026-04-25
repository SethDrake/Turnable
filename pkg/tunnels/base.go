package tunnels

import (
	"context"
	"io"
	"log/slog"
	"net"

	"github.com/theairblow/turnable/pkg/common"
	"github.com/theairblow/turnable/pkg/config"
)

// AcceptedClient is a local client connection accepted from a tunnel
type AcceptedClient struct {
	Stream io.ReadWriteCloser // Bidirectional connection to the local client
	Close  func() error       // Called to release any resources associated with the client
}

// Handler provides local client acceptance and remote route dialing
type Handler interface {
	ID() string                                                                 // Returns the unique ID of this handler
	Open(ctx context.Context, socketType string) (<-chan AcceptedClient, error) // Starts accepting local clients
	Connect(ctx context.Context, route *config.Route) (net.Conn, error)         // Connects to a remote route
	SetLogger(log *slog.Logger)                                                 // Changes the slog logger instance
}

// Handlers represents tunnel handler registry.
var Handlers = common.NewRegistry[Handler]()

// init registers the default socket tunnel handler
func init() {
	Handlers.Register(&SocketHandler{})
}

// GetHandler fetches a tunnel handler by its string ID.
func GetHandler(name string) (Handler, error) {
	return Handlers.Get(name)
}

// ListHandlers lists all registered tunnel handler IDs.
func ListHandlers() []string {
	return Handlers.List()
}
