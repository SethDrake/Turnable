package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

// Resolver represents the global DNS resolver instance
var Resolver rdns.Resolver

// providers is a list of DNS providers
var providers = []dnsProvider{
	{"https://common.dot.dns.yandex.net/dns-query", "common.dot.dns.yandex.net", []string{"77.88.8.8", "77.88.8.1"}},
	{"https://secure.dot.dns.yandex.net/dns-query", "secure.dot.dns.yandex.net", []string{"77.88.8.88", "77.88.8.2"}},
	{"https://family.dot.dns.yandex.net/dns-query", "family.dot.dns.yandex.net", []string{"77.88.8.7", "77.88.8.3"}},
	{"https://dns.google/dns-query", "dns.google", []string{"8.8.8.8", "8.8.4.4"}},
	{"https://cloudflare-dns.com/dns-query", "cloudflare-dns.com", []string{"1.1.1.1", "1.0.0.1"}},
}

// dnsProvider holds the metadata needed to build both a DoH and a DoT client
type dnsProvider struct {
	dohURL  string
	dotHost string
	rawIPs  []string
}

// init creates the global DNS resolver and forces Go's pure-Go DNS implementation
// so the custom resolver is used everywhere, not just for HTTP.
func init() {
	dohResolvers, err := buildDoHResolvers()
	if err != nil {
		panic(err)
	}

	dotResolvers, err := buildDoTResolvers()
	if err != nil {
		panic(err)
	}

	plainResolvers, err := buildPlainResolvers()
	if err != nil {
		panic(err)
	}

	dohGroup := rdns.NewRoundRobin("doh-group", dohResolvers...)
	dotGroup := rdns.NewRoundRobin("dot-group", dotResolvers...)
	plainGroup := rdns.NewRoundRobin("plain-group", plainResolvers...)

	Resolver = rdns.NewFailBack("failback", rdns.FailBackOptions{
		ServfailError: true,
	}, dohGroup, dotGroup, plainGroup)
}

// buildDoHResolvers creates one DoH client per provider
func buildDoHResolvers() ([]rdns.Resolver, error) {
	var resolvers []rdns.Resolver
	for _, p := range providers {
		for i, ip := range p.rawIPs {
			id := fmt.Sprintf("doh-%s-%d", p.dotHost, i)
			c, err := rdns.NewDoHClient(id, p.dohURL, rdns.DoHClientOptions{
				BootstrapAddr: ip,
			})
			if err != nil {
				return nil, fmt.Errorf("provider %s ip %s: %w", p.dotHost, ip, err)
			}
			resolvers = append(resolvers, c)
		}
	}
	return resolvers, nil
}

// buildDoTResolvers creates one DoT client per provider IP
func buildDoTResolvers() ([]rdns.Resolver, error) {
	var resolvers []rdns.Resolver
	for _, p := range providers {
		for i, ip := range p.rawIPs {
			id := fmt.Sprintf("dot-%s-%d", p.dotHost, i)
			endpoint := net.JoinHostPort(p.dotHost, "853")
			c, err := rdns.NewDoTClient(id, endpoint, rdns.DoTClientOptions{
				BootstrapAddr: ip,
				TLSConfig:     &tls.Config{},
			})
			if err != nil {
				return nil, fmt.Errorf("provider %s ip %s: %w", p.dotHost, ip, err)
			}
			resolvers = append(resolvers, c)
		}
	}
	return resolvers, nil
}

// buildPlainResolvers creates one plain UDP DNS client per rawIPs IP
func buildPlainResolvers() ([]rdns.Resolver, error) {
	seen := make(map[string]bool)
	var resolvers []rdns.Resolver
	for _, p := range providers {
		for _, ip := range p.rawIPs {
			if seen[ip] {
				continue
			}
			seen[ip] = true
			id := fmt.Sprintf("plain-%s", ip)
			endpoint := net.JoinHostPort(ip, "53")
			c, err := rdns.NewDNSClient(id, endpoint, "udp", rdns.DNSClientOptions{})
			if err != nil {
				return nil, fmt.Errorf("plain DNS %s: %w", ip, err)
			}
			resolvers = append(resolvers, c)
		}
	}
	return resolvers, nil
}

// Lookup resolves a hostname globally using the package level resolver
func Lookup(host string) ([]net.IP, error) {
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(host), dns.TypeA)
	q.RecursionDesired = true

	resp, err := Resolver.Resolve(q, rdns.ClientInfo{})
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

	if resp6, err := Resolver.Resolve(q6, rdns.ClientInfo{}); err == nil {
		for _, rr := range resp6.Answer {
			if v, ok := rr.(*dns.AAAA); ok {
				ips = append(ips, v.AAAA)
			}
		}
	}

	return ips, nil
}

// ResolverDialContext returns a DialContext function for use in http.Transport
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
