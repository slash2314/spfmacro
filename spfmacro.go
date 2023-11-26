// Package spfmacro is a CoreDNS plugin that allows to serve spf macros.
package spfmacro

import (
	"context"
	"fmt"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"golang.org/x/time/rate"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const DnsLookupAddr = "9.9.9.9:53"

// Define log to be a logger with the plugin name in it. This way we can just use log.Info and
// friends to log.
var log = clog.NewWithPlugin("spfmacro")

// SPFMacro is a plugin to serve SPF Macro compatible DNS
type SPFMacro struct {
	Next    plugin.Handler
	Domains []string
	Subnets []net.IPNet
	M       sync.RWMutex
	L       *rate.Limiter
}

// ServeDNS implements the plugin.Handler interface. This method gets called when spfmacro is used
// in a Server.
func (e *SPFMacro) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{
		Req: r,
		W:   w,
	}

	log.Debug("Received response")

	// Wrap.
	pw := NewResponsePrinter(w)

	// Export metric with the server label set to the current server handling the request.
	requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()
	txt := &dns.Msg{}

	txt.SetReply(r)
	txt.Authoritative = true

	z := state.Zone
	ip := state.IP()
	log.Debugf("Zone: %s IP: %s, Req: %v\n", z, ip, *state.Req)
	ipPattern := IPPattern.FindString(state.Req.String())
	if state.Family() == 1 && state.Type() == "TXT" && e.ValidateSPFMacroIP(ipPattern) {
		rr := e.CreateAcceptSPFRecord(state)
		txt.Answer = []dns.RR{rr}
		w.WriteMsg(txt)
	}
	return plugin.NextOrFailure(e.Name(), e.Next, ctx, pw, r)
}

func (e *SPFMacro) ValidateSPFMacroIP(requestPattern string) bool {
	if requestPattern == "" {
		return false
	}
	parseIP := net.ParseIP(requestPattern)
	e.M.RLock()
	spfMacroCheck := false
	log.Debugf("Number of Subnets: %d\n", len(e.Subnets))
	for _, subnet := range e.Subnets {
		if subnet.Contains(parseIP) {
			spfMacroCheck = true
			break
		}
	}
	e.M.RUnlock()
	return spfMacroCheck
}

func (e *SPFMacro) CreateAcceptSPFRecord(state request.Request) dns.RR {
	//var rr dns.RR
	rr := dns.TXT{
		Hdr: dns.RR_Header{
			Name:   state.QName(),
			Rrtype: dns.TypeTXT,
			Class:  state.QClass(),
			Ttl:    30,
		},
		Txt: []string{`v=spf1 -all`},
	}
	return &rr
}

type SPFEntry struct {
	Type  uint16
	Value string
}

func ParseSPFEntry(domain string) SPFEntry {
	domainSplit := strings.Split(domain, ":")
	var dnsType uint16
	switch domainSplit[0] {
	case "txt", "include":
		dnsType = dns.TypeTXT
	case "a":
		dnsType = dns.TypeA
	default:
		dnsType = dns.TypeTXT
	}
	return SPFEntry{
		Type:  dnsType,
		Value: domainSplit[1],
	}
}
func RetrieveSPFIPs(domains []string) ([]net.IPNet, error) {
	//fmt.Println("Processing ", domains)

	queriedSubnets := make([]net.IPNet, 0)
	c := new(dns.Client)
	c.ReadTimeout = 10 * time.Second
	c.DialTimeout = 10 * time.Second
	for _, domain := range domains {
		if !strings.Contains(domain, ":") {
			continue
		}
		m := new(dns.Msg)
		entry := ParseSPFEntry(domain)
		m.SetQuestion(fmt.Sprintf("%s.", entry.Value), entry.Type)
		m.RecursionDesired = true

		resp, _, err := c.Exchange(m, DnsLookupAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to dns for %s: %s", entry.Value, err)
		}

		for _, answer := range resp.Answer {
			parsedIPs := make([]net.IPNet, 0)
			if _, ok := answer.(*dns.A); ok {

				parsedIPs = append(parsedIPs, ParseIPs(answer.(*dns.A).A.String())...)
			} else {
				subnets := ParseIPs(answer.String())
				parsedIPs = append(parsedIPs, subnets...)
				newDomains := DomainPattern.FindAllString(answer.String(), -1)
				for _, newDomain := range newDomains {
					ipCidr, err := RetrieveSPFIPs([]string{newDomain})
					if err != nil {
						log.Error(err)
						return nil, err
					}
					parsedIPs = append(parsedIPs, ipCidr...)
				}
			}

			if len(parsedIPs) > 0 {
				//fmt.Printf("Domain: %s had %d IPs\n", domain, len(parsedIPs))
				queriedSubnets = append(queriedSubnets, parsedIPs...)
			}
		}
	}
	return queriedSubnets, nil
}

var CIDRPattern = regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/?\d{0,2})`)
var IPPattern = regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)

// var IPWithDomainPattern = regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\.([((www\.)?a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*))\._`)
var DomainPattern = regexp.MustCompile(`((txt|a|include):[((www\.)?a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*))`)

func ParseIPs(queryResult string) []net.IPNet {
	ipMatches := CIDRPattern.FindAllString(queryResult, -1)
	//fmt.Println(ipMatches)
	ipNetworks := make([]net.IPNet, 0)
	for _, ipStr := range ipMatches {
		// Handle the case of bare IP addresses
		if !strings.Contains(ipStr, "/") {
			ipStr += "/32"
		}
		_, ipnet, err := net.ParseCIDR(ipStr)
		if err != nil {
			fmt.Printf("There was an issue parsing %s\n", ipStr)
			continue
		}
		ipNetworks = append(ipNetworks, *ipnet)
	}
	return ipNetworks

}

// Name implements the Handler interface.
func (e *SPFMacro) Name() string { return "spfmacro" }

// ResponsePrinter wrap a dns.ResponseWriter and will write example to standard output when WriteMsg is called.
type ResponsePrinter struct {
	dns.ResponseWriter
}

// NewResponsePrinter returns ResponseWriter.
func NewResponsePrinter(w dns.ResponseWriter) *ResponsePrinter {
	return &ResponsePrinter{ResponseWriter: w}
}

// WriteMsg calls the underlying ResponseWriter's WriteMsg method and prints "example" to standard output.
func (r *ResponsePrinter) WriteMsg(res *dns.Msg) error {
	return r.ResponseWriter.WriteMsg(res)
}
