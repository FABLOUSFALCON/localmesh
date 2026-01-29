// Package gateway provides a local DNS server for the LocalMesh gateway.
// This solves the mDNS problem on Android by running a proper DNS server
// that responds to queries for our custom domain (e.g., campus.local)
package gateway

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// DNSServer runs a local DNS server that resolves custom domains
// and forwards other queries to upstream DNS servers
type DNSServer struct {
	domain    string   // e.g., "campus.local"
	ip        net.IP   // IP to return for our domain
	port      int      // DNS port (usually 53)
	upstream  []string // Upstream DNS servers for forwarding
	server    *dns.Server
	logger    *slog.Logger
	isRunning bool
}

// DNSConfig configures the DNS server
type DNSConfig struct {
	// Domain is the local domain to resolve (without trailing dot)
	// e.g., "campus.local"
	Domain string

	// IP is the IP address to return for the domain
	IP net.IP

	// Port is the DNS server port (default 53, but can use 5353 for non-root)
	Port int

	// Upstream DNS servers for forwarding other queries
	Upstream []string

	// Logger for logging
	Logger *slog.Logger
}

// DefaultDNSConfig returns sensible defaults
func DefaultDNSConfig() DNSConfig {
	return DNSConfig{
		Domain:   "campus.local",
		Port:     53,
		Upstream: []string{"8.8.8.8:53", "1.1.1.1:53"},
		Logger:   slog.Default(),
	}
}

// NewDNSServer creates a new DNS server
func NewDNSServer(cfg DNSConfig) *DNSServer {
	return &DNSServer{
		domain:   strings.ToLower(cfg.Domain),
		ip:       cfg.IP,
		port:     cfg.Port,
		upstream: cfg.Upstream,
		logger:   cfg.Logger,
	}
}

// Start begins the DNS server
func (d *DNSServer) Start() error {
	if d.ip == nil {
		return fmt.Errorf("no IP address configured for DNS")
	}

	// Create DNS handler
	mux := dns.NewServeMux()

	// Handle our domain
	mux.HandleFunc(d.domain+".", d.handleLocal)

	// Handle subdomains of our domain
	mux.HandleFunc("."+d.domain+".", d.handleLocal)

	// Forward everything else
	mux.HandleFunc(".", d.handleForward)

	// Create server - bind to specific IP to avoid conflicts with systemd-resolved
	// which binds to 127.0.0.53:53
	addr := fmt.Sprintf("%s:%d", d.ip.String(), d.port)
	d.server = &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: mux,
	}

	// Start in goroutine
	go func() {
		d.isRunning = true
		if err := d.server.ListenAndServe(); err != nil {
			if d.isRunning {
				d.logger.Error("DNS server error", "error", err)
			}
		}
	}()

	d.logger.Info("DNS server started",
		"domain", d.domain,
		"ip", d.ip.String(),
		"port", d.port,
	)

	return nil
}

// handleLocal handles queries for our local domain
func (d *DNSServer) handleLocal(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		d.logger.Debug("DNS query for local domain",
			"name", q.Name,
			"type", dns.TypeToString[q.Qtype],
		)

		switch q.Qtype {
		case dns.TypeA:
			// Return our IP for A record queries
			if ip4 := d.ip.To4(); ip4 != nil {
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    300, // 5 minutes
					},
					A: ip4,
				}
				msg.Answer = append(msg.Answer, rr)
			}

		case dns.TypeAAAA:
			// Return empty for AAAA (we don't do IPv6)
			// This prevents slow fallback

		case dns.TypeANY:
			// Return A record for ANY queries
			if ip4 := d.ip.To4(); ip4 != nil {
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    300,
					},
					A: ip4,
				}
				msg.Answer = append(msg.Answer, rr)
			}
		}
	}

	w.WriteMsg(msg)
}

// handleForward forwards queries to upstream DNS
func (d *DNSServer) handleForward(w dns.ResponseWriter, r *dns.Msg) {
	// Try each upstream until one works
	client := new(dns.Client)
	client.Timeout = 2 * 1e9 // 2 seconds

	for _, upstream := range d.upstream {
		resp, _, err := client.Exchange(r, upstream)
		if err == nil {
			w.WriteMsg(resp)
			return
		}
	}

	// All upstreams failed, return SERVFAIL
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Rcode = dns.RcodeServerFailure
	w.WriteMsg(msg)
}

// Stop stops the DNS server
func (d *DNSServer) Stop() {
	d.isRunning = false
	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
		d.logger.Info("DNS server stopped")
	}
}

// Domain returns the configured domain
func (d *DNSServer) Domain() string {
	return d.domain
}

// IP returns the configured IP
func (d *DNSServer) IP() net.IP {
	return d.ip
}

// IsRunning returns whether the server is running
func (d *DNSServer) IsRunning() bool {
	return d.isRunning
}
