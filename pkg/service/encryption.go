package service

import (
	"bytes"
	"crypto/cipher"
	"crypto/mlkem"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/theairblow/turnable/pkg/internal/connection"
	pb "github.com/theairblow/turnable/pkg/service/proto"
)

const (
	serviceVersion = uint32(1) // Service protocol version
)

// KeyPair holds a parsed ML-KEM768 keypair for service encryption
type KeyPair struct {
	pubKeyBytes []byte
	privKey     *mlkem.DecapsulationKey768
}

// NewKeyPair parses a base64-encoded ML-KEM768 keypair
func NewKeyPair(pubKeyB64, privKeyB64 string) (*KeyPair, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	if _, err := mlkem.NewEncapsulationKey768(pubBytes); err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	privBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	privKey, err := mlkem.NewDecapsulationKey768(privBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return &KeyPair{pubKeyBytes: pubBytes, privKey: privKey}, nil
}

// serviceEncLayer provides directional AES-256-GCM-12 encryption over byte slices
type serviceEncLayer struct {
	readAEAD    cipher.AEAD
	writeAEAD   cipher.AEAD
	readPrefix  [4]byte
	writePrefix [4]byte
	writeSeq    uint64
}

// newServiceEncLayer derives directional AEAD contexts from the shared key
func newServiceEncLayer(sharedKey []byte, isClient bool) *serviceEncLayer {
	clientKey, clientPrefix := connection.DeriveTunnelMaterial(sharedKey, "client->server")
	serverKey, serverPrefix := connection.DeriveTunnelMaterial(sharedKey, "server->client")

	var readKey, writeKey []byte
	var readPrefix, writePrefix [4]byte
	if isClient {
		writeKey, writePrefix = clientKey, clientPrefix
		readKey, readPrefix = serverKey, serverPrefix
	} else {
		writeKey, writePrefix = serverKey, serverPrefix
		readKey, readPrefix = clientKey, clientPrefix
	}

	rAEAD, _ := connection.NewTunnelAEAD(readKey)
	wAEAD, _ := connection.NewTunnelAEAD(writeKey)

	return &serviceEncLayer{
		readAEAD:    rAEAD,
		writeAEAD:   wAEAD,
		readPrefix:  readPrefix,
		writePrefix: writePrefix,
	}
}

// serverHandshake sends a ServerHello and negotiates encryption if necessary
func serverHandshake(nc net.Conn, kp *KeyPair, allowedKeys [][]byte) (*serviceEncLayer, error) {
	hello := &pb.ServerHello{
		Magic:        "TSVC",
		Version:      serviceVersion,
		AuthRequired: kp != nil,
	}
	if kp != nil {
		hello.PublicKey = kp.pubKeyBytes
	}

	if err := writeFramed(nc, hello); err != nil {
		return nil, fmt.Errorf("write server hello: %w", err)
	}
	if kp == nil {
		return nil, nil
	}

	var clientHello pb.ClientHello
	if err := readFramed(nc, &clientHello); err != nil {
		return nil, fmt.Errorf("read client hello: %w", err)
	}

	if len(allowedKeys) > 0 {
		allowed := false
		for _, k := range allowedKeys {
			if bytes.Equal(k, clientHello.PublicKey) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("client public key not in allowlist")
		}
	}

	sharedKey, err := kp.privKey.Decapsulate(clientHello.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decapsulate: %w", err)
	}

	return newServiceEncLayer(sharedKey, false), nil
}

// encrypt encrypts a plain byte slice
func (e *serviceEncLayer) encrypt(plain []byte) []byte {
	var nonce [12]byte
	copy(nonce[:4], e.writePrefix[:])
	binary.BigEndian.PutUint64(nonce[4:], e.writeSeq)
	e.writeSeq++
	out := make([]byte, 8, 8+len(plain)+e.writeAEAD.Overhead())
	copy(out, nonce[4:])
	return e.writeAEAD.Seal(out, nonce[:], plain, nil)
}

// decrypt decrypts a plain byte slice
func (e *serviceEncLayer) decrypt(data []byte) ([]byte, error) {
	if len(data) < 8+e.readAEAD.Overhead() {
		return nil, fmt.Errorf("encrypted payload too short (%d bytes)", len(data))
	}

	var nonce [12]byte
	copy(nonce[:4], e.readPrefix[:])
	copy(nonce[4:], data[:8])
	plain, err := e.readAEAD.Open(nil, nonce[:], data[8:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plain, nil
}
