package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// providers is the list of DNS providers used for DoH, DoT, and plain UDP fallback
var providers = []struct {
	dohURL  string
	dotHost string
	rawIPs  []string
}{
	{"https://common.dot.dns.yandex.net/dns-query", "common.dot.dns.yandex.net", []string{"77.88.8.8", "77.88.8.1"}},
	{"https://secure.dot.dns.yandex.net/dns-query", "secure.dot.dns.yandex.net", []string{"77.88.8.88", "77.88.8.2"}},
	{"https://family.dot.dns.yandex.net/dns-query", "family.dot.dns.yandex.net", []string{"77.88.8.7", "77.88.8.3"}},
	{"https://dns.google/dns-query", "dns.google", []string{"8.8.8.8", "8.8.4.4"}},
	{"https://cloudflare-dns.com/dns-query", "cloudflare-dns.com", []string{"1.1.1.1", "1.0.0.1"}},
}

// globalResolver is the package-level DNS resolver used by Lookup
var globalResolver resolver

// init builds the global resolver chain from all configured providers
func init() {
	var dohGroup, dotGroup, plainGroup roundRobin
	seen := map[string]bool{}

	for _, p := range providers {
		for _, ip := range p.rawIPs {
			dohGroup.resolvers = append(dohGroup.resolvers, newDoHClient(p.dohURL, ip, p.dotHost))
			dotGroup.resolvers = append(dotGroup.resolvers, newDoTClient(ip, p.dotHost))
			if !seen[ip] {
				seen[ip] = true
				plainGroup.resolvers = append(plainGroup.resolvers, newPlainClient(ip))
			}
		}
	}

	globalResolver = &failBack{resolvers: []resolver{&dohGroup, &dotGroup, &plainGroup}}
}

// resolver resolves a single DNS question
type resolver interface {
	resolve(q *dns.Msg) (*dns.Msg, error)
}

// roundRobin distributes queries across a set of resolvers in round-robin order
type roundRobin struct {
	resolvers []resolver
	idx       atomic.Uint64
}

// resolve picks the next resolver in round-robin order and delegates the query
func (r *roundRobin) resolve(q *dns.Msg) (*dns.Msg, error) {
	if len(r.resolvers) == 0 {
		return nil, fmt.Errorf("no resolvers")
	}
	i := int(r.idx.Add(1)-1) % len(r.resolvers)
	return r.resolvers[i].resolve(q)
}

// failBack tries each resolver in order, falling back on error or SERVFAIL
type failBack struct {
	resolvers []resolver
}

// resolve tries each resolver in order and returns the first successful non-SERVFAIL response
func (f *failBack) resolve(q *dns.Msg) (*dns.Msg, error) {
	var lastErr error
	for _, r := range f.resolvers {
		resp, err := r.resolve(q)
		if err == nil && resp.Rcode != dns.RcodeServerFailure {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("SERVFAIL")
		}
	}
	return nil, lastErr
}

// dohClient resolves DNS-over-HTTPS
type dohClient struct {
	url        string
	httpClient *http.Client
}

// newDoHClient creates a DoH client that dials bootstrapIP directly to avoid circular DNS
func newDoHClient(dohURL, bootstrapIP, hostname string) *dohClient {
	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(addr)
			if port == "" {
				port = "443"
			}
			conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(bootstrapIP, port))
			if err != nil {
				return nil, err
			}
			tlsConn := tls.Client(conn, &tls.Config{ServerName: hostname})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, err
			}
			return tlsConn, nil
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	return &dohClient{
		url:        dohURL,
		httpClient: &http.Client{Transport: transport, Timeout: 10 * time.Second},
	}
}

// resolve sends the DNS query as a POST request to the DoH endpoint
func (c *dohClient) resolve(q *dns.Msg) (*dns.Msg, error) {
	packed, err := q.Pack()
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Post(c.url, "application/dns-message", bytes.NewReader(packed))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	msg := new(dns.Msg)
	return msg, msg.Unpack(body)
}

// dotClient resolves DNS-over-TLS
type dotClient struct {
	addr   string
	client *dns.Client
}

// newDoTClient creates a DoT client dialing bootstrapIP on port 853
func newDoTClient(bootstrapIP, hostname string) *dotClient {
	return &dotClient{
		addr: net.JoinHostPort(bootstrapIP, "853"),
		client: &dns.Client{
			Net:     "tcp-tls",
			Timeout: 10 * time.Second,
			TLSConfig: &tls.Config{
				ServerName: hostname,
			},
		},
	}
}

// resolve sends the DNS query over a TLS connection
func (c *dotClient) resolve(q *dns.Msg) (*dns.Msg, error) {
	resp, _, err := c.client.Exchange(q, c.addr)
	return resp, err
}

// plainClient resolves DNS over plain UDP
type plainClient struct {
	addr   string
	client *dns.Client
}

// newPlainClient creates a plain UDP DNS client dialing ip on port 53
func newPlainClient(ip string) *plainClient {
	return &plainClient{
		addr:   net.JoinHostPort(ip, "53"),
		client: &dns.Client{Net: "udp", Timeout: 5 * time.Second},
	}
}

// resolve sends the DNS query over plain UDP
func (c *plainClient) resolve(q *dns.Msg) (*dns.Msg, error) {
	resp, _, err := c.client.Exchange(q, c.addr)
	return resp, err
}

// Lookup resolves a hostname to IP addresses using the global resolver
func Lookup(host string) ([]net.IP, error) {
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(host), dns.TypeA)
	q.RecursionDesired = true

	resp, err := globalResolver.resolve(q)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", host, err)
	}

	var ips []net.IP
	for _, rr := range resp.Answer {
		switch v := rr.(type) {
		case *dns.A:
			ips = append(ips, v.A)
		case *dns.AAAA:
			ips = append(ips, v.AAAA)
		}
	}

	q6 := new(dns.Msg)
	q6.SetQuestion(dns.Fqdn(host), dns.TypeAAAA)
	q6.RecursionDesired = true
	if resp6, err := globalResolver.resolve(q6); err == nil {
		for _, rr := range resp6.Answer {
			if v, ok := rr.(*dns.AAAA); ok {
				ips = append(ips, v.AAAA)
			}
		}
	}

	return ips, nil
}

// ResolverDialContext returns a DialContext function that uses the global DNS resolver
func ResolverDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if net.ParseIP(host) != nil {
			return dialer.DialContext(ctx, network, addr)
		}
		ips, err := Lookup(host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses for %s", host)
		}
		var conn net.Conn
		for _, ip := range ips {
			conn, err = dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, err
	}
}
