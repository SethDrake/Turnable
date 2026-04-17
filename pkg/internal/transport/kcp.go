package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	kcp "github.com/xtaci/kcp-go/v5"
)

const (
	kcpWindowSize    = 512             // KCP send/receive window size
	kcpUpdateMs      = 40              // KCP update interval in milliseconds
	kcpResend        = 2               // Fast resend after 2 duplicate ACKs
	kcpDisableCC     = 1               // Disable KCP congestion control
	kcpMTU           = 1200            // Maximum KCP MTU used for this transport
	kcpReadWriteBuff = 2 * 1024 * 1024 // Read/write buffer size for the session
	kcpConversation  = 1               // Conversation ID for this transport channel
)

// KCPHandler represents a KCP transport handler
type KCPHandler struct{}

// ID returns the unique ID of this handler
func (D *KCPHandler) ID() string {
	return "kcp"
}

// WrapClient wraps a client packet connection
func (D *KCPHandler) WrapClient(conn net.PacketConn) (io.ReadWriteCloser, error) {
	return wrapKCP(conn)
}

// WrapServer wraps a server packet connection
func (D *KCPHandler) WrapServer(conn net.PacketConn) (io.ReadWriteCloser, error) {
	return wrapKCP(conn)
}

// wrapKCP initializes a KCP session directly over a net.PacketConn
func wrapKCP(pc net.PacketConn) (io.ReadWriteCloser, error) {
	session, err := kcp.NewConn3(kcpConversation, pc.LocalAddr(), nil, 0, 0, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kcp session: %w", err)
	}

	session.SetNoDelay(1, kcpUpdateMs, kcpResend, kcpDisableCC)
	session.SetWindowSize(kcpWindowSize, kcpWindowSize)
	session.SetACKNoDelay(true)
	session.SetStreamMode(true)
	session.SetWriteDelay(false)
	_ = session.SetReadBuffer(kcpReadWriteBuff)
	_ = session.SetWriteBuffer(kcpReadWriteBuff)
	_ = session.SetMtu(kcpMTU)

	return &managedKCPConn{packetConn: pc, session: session}, nil
}

// managedKCPConn represents a managed KCP connection
type managedKCPConn struct {
	packetConn net.PacketConn
	session    *kcp.UDPSession

	closeMu  sync.Mutex
	isClosed bool
}

// Read forwards payload bytes out of the underlying KCP session
func (c *managedKCPConn) Read(p []byte) (int, error) {
	return c.session.Read(p)
}

// Write forwards payload bytes into the underlying KCP session
func (c *managedKCPConn) Write(p []byte) (int, error) {
	return c.session.Write(p)
}

// Close closes both the KCP session and the underlying packet connection
func (c *managedKCPConn) Close() error {
	c.closeMu.Lock()
	if c.isClosed {
		c.closeMu.Unlock()
		return nil
	}
	c.isClosed = true
	c.closeMu.Unlock()

	return errors.Join(
		func() error {
			if c.session == nil {
				return nil
			}
			return c.session.Close()
		}(),
		func() error {
			if c.packetConn == nil {
				return nil
			}
			return c.packetConn.Close()
		}(),
	)
}
