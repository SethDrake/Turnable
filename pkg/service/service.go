package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/theairblow/turnable/pkg/common"
	"github.com/theairblow/turnable/pkg/config/providers"
	pb "github.com/theairblow/turnable/pkg/service/proto"
)

// Service manages VPN instances and exposes them over a protobuf socket protocol
type Service struct {
	mu        sync.RWMutex
	instances map[string]*Instance

	log         *slog.Logger
	relay       *LogRelayHandler
	keyPair     *KeyPair
	allowedKeys [][]byte

	listenersMu sync.Mutex
	listeners   []net.Listener
}

// NewService creates a new Service instance
func NewService(serverPubB64, serverPrivB64 string, allowedClientKeysB64 ...string) (*Service, error) {
	var kp *KeyPair
	if serverPubB64 != "" || serverPrivB64 != "" {
		var err error
		kp, err = NewKeyPair(serverPubB64, serverPrivB64)
		if err != nil {
			return nil, fmt.Errorf("parse server keypair: %w", err)
		}
	}
	if len(allowedClientKeysB64) > 0 && kp == nil {
		return nil, errors.New("allowed client keys require a server keypair")
	}

	allowedKeys := make([][]byte, 0, len(allowedClientKeysB64))
	for _, k := range allowedClientKeysB64 {
		b, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("decode allowed client key: %w", err)
		}
		allowedKeys = append(allowedKeys, b)
	}

	relay := newLogRelayHandler(slog.Default().Handler())
	common.SetLogHandler(relay)
	return &Service{
		instances:   make(map[string]*Instance),
		log:         slog.Default(),
		relay:       relay,
		keyPair:     kp,
		allowedKeys: allowedKeys,
	}, nil
}

// ListenTCP accepts service connections on the given TCP address
func (s *Service) ListenTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}

	s.listenersMu.Lock()
	s.listeners = append(s.listeners, ln)
	s.listenersMu.Unlock()
	go s.accept(ln)
	return nil
}

// ListenUnix accepts service connections on the given Unix socket path
func (s *Service) ListenUnix(path string) error {
	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}

	s.listenersMu.Lock()
	s.listeners = append(s.listeners, ln)
	s.listenersMu.Unlock()
	go s.accept(ln)
	return nil
}

// Stop closes all listeners and stops all managed instances
func (s *Service) Stop() {
	s.listenersMu.Lock()
	for _, ln := range s.listeners {
		_ = ln.Close()
	}
	s.listeners = nil
	s.listenersMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inst := range s.instances {
		_ = inst.Stop()
	}
}

// accept accepts incoming connections from a listener and serves each one
func (s *Service) accept(ln net.Listener) {
	for {
		nc, err := ln.Accept()
		if err != nil {
			return
		}

		go newConn(s, nc).serve()
	}
}

// startServer creates and starts a Turnable server instance
func (s *Service) startServer(req *pb.StartServerRequest) (string, error) {
	srv, provider, err := buildServerInstance(req)
	if err != nil {
		return "", err
	}

	tunnel, err := buildTunnelHandler(req.Tunnel)
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	srv.SetLogger(s.log.With("server_id", id))
	if err := srv.Start(tunnel); err != nil {
		return "", fmt.Errorf("start server: %w", err)
	}

	s.mu.Lock()
	s.instances[id] = &Instance{ID: id, server: srv, provider: provider}
	s.mu.Unlock()
	return id, nil
}

// startClient creates and starts a Turnable client instance
func (s *Service) startClient(req *pb.StartClientRequest) (string, error) {
	cli, err := buildClientInstance(req)
	if err != nil {
		return "", err
	}

	tunnel, err := buildTunnelHandler(req.Tunnel)
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	cli.SetLogger(s.log.With("client_id", id))
	if err := cli.Start(tunnel); err != nil {
		return "", fmt.Errorf("start client: %w", err)
	}

	s.mu.Lock()
	s.instances[id] = &Instance{ID: id, client: cli}
	s.mu.Unlock()
	return id, nil
}

// stopInstance stops and removes an instance by ID
func (s *Service) stopInstance(id string) error {
	s.mu.Lock()
	inst, ok := s.instances[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("instance %q not found", id)
	}
	delete(s.instances, id)
	s.mu.Unlock()
	return inst.Stop()
}

// updateProvider updates config provider for an instance
func (s *Service) updateProvider(id string, cfg *pb.ProviderConfig) error {
	s.mu.RLock()
	inst, ok := s.instances[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}

	var data string
	if cfg != nil {
		if v := cfg.Args["data"]; v != nil {
			if sv, ok := v.Value.(*pb.ParamValue_StringVal); ok {
				data = sv.StringVal
			}
		}
	}

	switch p := inst.provider.(type) {
	case *providers.JSONProvider:
		return p.UpdateFromJSON(data)
	default:
		return fmt.Errorf("current config provider does not support updating")
	}
}

// listInstances returns info for all managed instances
func (s *Service) listInstances() []*pb.InstanceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]*pb.InstanceInfo, 0, len(s.instances))
	for _, inst := range s.instances {
		infos = append(infos, inst.Info())
	}
	return infos
}
