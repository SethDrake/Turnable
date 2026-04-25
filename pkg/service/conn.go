package service

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	pb "github.com/theairblow/turnable/pkg/service/proto"
	"google.golang.org/protobuf/proto"
)

// conn handles a single service client connection
type conn struct {
	svc     *Service
	nc      net.Conn
	enc     *serviceEncLayer
	writeCh chan *pb.Response
}

// newConn allocates a conn for an incoming connection
func newConn(svc *Service, nc net.Conn) *conn {
	return &conn{
		svc:     svc,
		nc:      nc,
		writeCh: make(chan *pb.Response, 64),
	}
}

// serve performs the handshake, subscribes to logs, then runs the IO loops
func (c *conn) serve() {
	defer c.svc.relay.broadcast.unsubscribe(c)
	defer c.nc.Close()

	enc, err := serverHandshake(c.nc, c.svc.keyPair, c.svc.allowedKeys)
	if err != nil {
		c.svc.log.Warn("service handshake failed", "remote", c.nc.RemoteAddr(), "error", err)
		return
	}
	c.enc = enc
	c.svc.relay.broadcast.subscribe(c)

	go c.writeLoop()
	c.readLoop()
}

// readLoop reads and dispatches requests until the connection closes
func (c *conn) readLoop() {
	defer close(c.writeCh)
	for {
		var req pb.Request
		if err := c.readMsg(&req); err != nil {
			return
		}

		resp, err := c.dispatch(&req)
		if err != nil {
			c.sendErr(err)
			return
		}

		if resp != nil {
			c.writeCh <- resp
		}
	}
}

// writeLoop drains channel and sends responses to the client
func (c *conn) writeLoop() {
	defer c.nc.Close()
	for resp := range c.writeCh {
		if err := c.writeMsg(resp); err != nil {
			return
		}
	}
}

// sendLog enqueues a log record without blocking
func (c *conn) sendLog(rec *pb.LogRecord) {
	select {
	case c.writeCh <- &pb.Response{Payload: &pb.Response_LogRecord{LogRecord: rec}}:
	default:
	}
}

// sendErr sends a fatal ErrorResponse
func (c *conn) sendErr(err error) {
	_ = c.writeMsg(&pb.Response{Payload: &pb.Response_Error{Error: &pb.ErrorResponse{Message: err.Error()}}})
}

// dispatch handles an incoming request and sends a response
func (c *conn) dispatch(req *pb.Request) (*pb.Response, error) {
	switch p := req.Payload.(type) {
	case *pb.Request_StartServer:
		id, err := c.svc.startServer(p.StartServer)
		resp := &pb.StartServerResponse{InstanceId: id}
		if err != nil {
			resp.Error = err.Error()
		}

		return &pb.Response{Payload: &pb.Response_StartServer{StartServer: resp}}, nil
	case *pb.Request_StartClient:
		id, err := c.svc.startClient(p.StartClient)
		resp := &pb.StartClientResponse{InstanceId: id}
		if err != nil {
			resp.Error = err.Error()
		}

		return &pb.Response{Payload: &pb.Response_StartClient{StartClient: resp}}, nil
	case *pb.Request_StopInstance:
		resp := &pb.StopInstanceResponse{}
		if err := c.svc.stopInstance(p.StopInstance.InstanceId); err != nil {
			resp.Error = err.Error()
		}

		return &pb.Response{Payload: &pb.Response_StopInstance{StopInstance: resp}}, nil
	case *pb.Request_UpdateProvider:
		if err := c.svc.updateProvider(p.UpdateProvider.InstanceId, p.UpdateProvider.Provider); err != nil {
			return nil, err
		}

		return &pb.Response{Payload: &pb.Response_UpdateProvider{UpdateProvider: &pb.UpdateProviderResponse{}}}, nil
	case *pb.Request_ListInstances:
		return &pb.Response{Payload: &pb.Response_ListInstances{ListInstances: &pb.ListInstancesResponse{
			Instances: c.svc.listInstances(),
		}}}, nil
	case *pb.Request_ValidateServerConfig:
		return &pb.Response{Payload: &pb.Response_ValidateServerConfig{
			ValidateServerConfig: handleValidateServerConfig(p.ValidateServerConfig),
		}}, nil
	case *pb.Request_ValidateClientConfig:
		return &pb.Response{Payload: &pb.Response_ValidateClientConfig{
			ValidateClientConfig: handleValidateClientConfig(p.ValidateClientConfig),
		}}, nil
	case *pb.Request_ConvertClientConfig:
		resp, err := handleConvertClientConfig(p.ConvertClientConfig)
		if err != nil {
			return nil, err
		}

		return &pb.Response{Payload: &pb.Response_ConvertClientConfig{ConvertClientConfig: resp}}, nil
	default:
		return nil, fmt.Errorf("unknown request type")
	}
}

// writeFramed writes a length-prefixed proto message without encryption
func writeFramed(nc net.Conn, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := nc.Write(lenBuf[:]); err != nil {
		return err
	}

	_, err = nc.Write(data)
	return err
}

// readFramed reads a length-prefixed proto message without decryption
func readFramed(nc net.Conn, msg proto.Message) error {
	var lenBuf [4]byte
	if _, err := io.ReadFull(nc, lenBuf[:]); err != nil {
		return err
	}

	size := binary.BigEndian.Uint32(lenBuf[:])
	if size > 8*1024*1024 {
		return fmt.Errorf("message too large: %d bytes", size)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(nc, data); err != nil {
		return err
	}

	return proto.Unmarshal(data, msg)
}

// writeMsg writes a length-prefixed, optionally encrypted proto message
func (c *conn) writeMsg(msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if c.enc != nil {
		data = c.enc.encrypt(data)
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := c.nc.Write(lenBuf[:]); err != nil {
		return err
	}

	_, err = c.nc.Write(data)
	return err
}

// readMsg reads a length-prefixed, optionally encrypted proto message
func (c *conn) readMsg(msg proto.Message) error {
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.nc, lenBuf[:]); err != nil {
		return err
	}

	size := binary.BigEndian.Uint32(lenBuf[:])
	if size > 8*1024*1024 {
		return fmt.Errorf("message too large: %d bytes", size)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(c.nc, data); err != nil {
		return err
	}

	if c.enc != nil {
		var err error
		data, err = c.enc.decrypt(data)
		if err != nil {
			return err
		}
	}

	return proto.Unmarshal(data, msg)
}
