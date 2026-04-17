package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pion/sctp"
)

const (
	sctpStreamID            = 0               // SCTP stream ID used by this transport
	sctpMaxReceiveBuffer    = 4 * 1024 * 1024 // Max SCTP receive buffer size
	sctpMaxMessageSize      = 64 * 1024       // Max SCTP message size accepted by the association
	sctpRetransmitTimeoutMs = 2000            // Maximum SCTP retransmit timeout in milliseconds
)

// SCTPHandler represents an SCTP transport handler
type SCTPHandler struct{}

// ID returns the unique ID of this handler
func (D *SCTPHandler) ID() string {
	return "sctp"
}

// WrapClient wraps a client packet connection
func (D *SCTPHandler) WrapClient(conn net.PacketConn) (io.ReadWriteCloser, error) {
	nc := &sctpPacketConnNetConn{pc: conn}
	assoc, err := sctp.Client(sctp.Config{
		Name:                 "turnable-transport-sctp-client",
		NetConn:              nc,
		MaxReceiveBufferSize: sctpMaxReceiveBuffer,
		MaxMessageSize:       sctpMaxMessageSize,
		RTOMax:               sctpRetransmitTimeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client sctp association: %w", err)
	}

	sctpStream, err := assoc.OpenStream(sctpStreamID, sctp.PayloadTypeWebRTCBinary)
	if err != nil {
		_ = assoc.Close()
		return nil, fmt.Errorf("failed to open sctp stream: %w", err)
	}

	return &managedSCTPConn{base: conn, assoc: assoc, stream: sctpStream}, nil
}

// WrapServer wraps a server packet connection
func (D *SCTPHandler) WrapServer(conn net.PacketConn) (io.ReadWriteCloser, error) {
	nc := &sctpPacketConnNetConn{pc: conn}
	assoc, err := sctp.Server(sctp.Config{
		Name:                 "turnable-transport-sctp-server",
		NetConn:              nc,
		MaxReceiveBufferSize: sctpMaxReceiveBuffer,
		MaxMessageSize:       sctpMaxMessageSize,
		RTOMax:               sctpRetransmitTimeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create server sctp association: %w", err)
	}

	sctpStream, err := assoc.AcceptStream()
	if err != nil {
		_ = assoc.Close()
		return nil, fmt.Errorf("failed to accept sctp stream: %w", err)
	}

	return &managedSCTPConn{base: conn, assoc: assoc, stream: sctpStream}, nil
}

// managedSCTPConn represents a managed SCTP connection
type managedSCTPConn struct {
	base   io.Closer
	assoc  *sctp.Association
	stream *sctp.Stream

	closeMu  sync.Mutex
	isClosed bool
}

// Read forwards payload bytes out of the underlying SCTP stream
func (c *managedSCTPConn) Read(p []byte) (int, error) {
	return c.stream.Read(p)
}

// Write forwards payload bytes into the underlying SCTP stream
func (c *managedSCTPConn) Write(p []byte) (int, error) {
	return c.stream.Write(p)
}

// Close closes the stream, association, and underlying transport.
func (c *managedSCTPConn) Close() error {
	c.closeMu.Lock()
	if c.isClosed {
		c.closeMu.Unlock()
		return nil
	}
	c.isClosed = true
	c.closeMu.Unlock()

	return errors.Join(
		func() error {
			if c.stream == nil {
				return nil
			}
			return c.stream.Close()
		}(),
		func() error {
			if c.assoc == nil {
				return nil
			}
			return c.assoc.Close()
		}(),
		func() error {
			if c.base == nil {
				return nil
			}
			return c.base.Close()
		}(),
	)
}

// sctpPacketConnNetConn adapts a net.PacketConn to net.Conn
type sctpPacketConnNetConn struct {
	pc net.PacketConn
}

// Read reads one packet from the underlying net.PacketConn
func (c *sctpPacketConnNetConn) Read(p []byte) (int, error) {
	n, _, err := c.pc.ReadFrom(p)
	return n, err
}

// Write sends one packet via the underlying net.PacketConn
func (c *sctpPacketConnNetConn) Write(p []byte) (int, error) {
	return c.pc.WriteTo(p, c.pc.LocalAddr())
}

// Close closes the underlying net.PacketConn
func (c *sctpPacketConnNetConn) Close() error {
	return c.pc.Close()
}

// LocalAddr returns the local address of the underlying net.PacketConn
func (c *sctpPacketConnNetConn) LocalAddr() net.Addr {
	return c.pc.LocalAddr()
}

// RemoteAddr returns a synthetic remote address
func (c *sctpPacketConnNetConn) RemoteAddr() net.Addr {
	return sctpDummyAddr{}
}

// SetDeadline forwards deadline configuration
func (c *sctpPacketConnNetConn) SetDeadline(t time.Time) error {
	return c.pc.SetDeadline(t)
}

// SetReadDeadline forwards read deadline configuration
func (c *sctpPacketConnNetConn) SetReadDeadline(t time.Time) error {
	return c.pc.SetReadDeadline(t)
}

// SetWriteDeadline forwards write deadline configuration
func (c *sctpPacketConnNetConn) SetWriteDeadline(t time.Time) error {
	return c.pc.SetWriteDeadline(t)
}

// sctpDummyAddr provides a synthetic remote address
type sctpDummyAddr struct{}

// Network returns the synthetic network name used for the SCTP transport
func (sctpDummyAddr) Network() string { return "sctp-transport" }

// String returns the synthetic address string used for the SCTP transport
func (sctpDummyAddr) String() string { return "sctp-transport" }
