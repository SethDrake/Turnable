package transport

import (
	"io"
	"net"
)

// NoneHandler represents a passthrough transport handler
type NoneHandler struct{}

// ID returns the unique ID of this handler
func (D *NoneHandler) ID() string {
	return "none"
}

// WrapClient wraps a client packet connection
func (D *NoneHandler) WrapClient(conn net.PacketConn) (io.ReadWriteCloser, error) {
	return &noneConn{pc: conn}, nil
}

// WrapServer wraps a server packet connection
func (D *NoneHandler) WrapServer(conn net.PacketConn) (io.ReadWriteCloser, error) {
	return &noneConn{pc: conn}, nil
}

// noneConn adapts a net.PacketConn to io.ReadWriteCloser
type noneConn struct {
	pc net.PacketConn
}

// Read reads one packet from the underlying net.PacketConn
func (c *noneConn) Read(p []byte) (int, error) {
	n, _, err := c.pc.ReadFrom(p)
	return n, err
}

// Write sends one packet via the underlying net.PacketConn
func (c *noneConn) Write(p []byte) (int, error) {
	return c.pc.WriteTo(p, c.pc.LocalAddr())
}

// Close closes the underlying net.PacketConn
func (c *noneConn) Close() error {
	return c.pc.Close()
}
