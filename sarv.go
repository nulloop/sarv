package sarv

import (
	"context"
	"fmt"
	"net"

	"github.com/miekg/dns"
)

var (
	// ErrAssignResolverDial happens when net.DefaultResolver.Dial has already been initialized
	ErrAssignResolverDial = fmt.Errorf("DefaultResolver.Dial is not nil")
)

// RouteDNS initialize and override internal net.DefaultResolver.Dial
// to route all DNS traffic to custom dns server
func RouteDNS(dnsAddr string) error {
	if net.DefaultResolver.Dial != nil {
		return ErrAssignResolverDial
	}

	net.DefaultResolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		dialer := net.Dialer{}

		if port == "53" {
			return dialer.DialContext(ctx, "udp", dnsAddr)
		}

		return dialer.DialContext(ctx, network, address)
	}

	return nil
}

// Server is an interface which simplified the process of working with DNS SRV
type Server interface {
	HandleSRV(req *dns.Msg) *dns.Msg
}

type handler struct {
	server         Server
	dnsDefaultAddr string
}

func (h *handler) routeDefaultDNS(w dns.ResponseWriter, r *dns.Msg) {
	// default dns
	c := new(dns.Client)
	msg, _, err := c.Exchange(r, h.dnsDefaultAddr)
	if err != nil {
		msg = &dns.Msg{}
		msg.SetReply(r)
	}

	w.WriteMsg(msg)
}

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	switch r.Question[0].Qtype {
	case dns.TypeSRV:
		msg := h.server.HandleSRV(r)
		if msg != nil {
			w.WriteMsg(msg)
			return
		}
		fallthrough
	default:
		h.routeDefaultDNS(w, r)
	}
}

// ServeAndHandleSRV a function which prepare the logic of
func ServeAndHandleSRV(dnsAddr, dnsDefaultAddr string, server Server) error {
	srv := &dns.Server{Addr: dnsAddr, Net: "udp"}
	srv.Handler = &handler{
		server:         server,
		dnsDefaultAddr: dnsDefaultAddr,
	}
	return srv.ListenAndServe()
}
