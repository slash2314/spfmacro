package spfmacro

import (
	"errors"
	"fmt"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"golang.org/x/time/rate"
	"strings"
	"sync"
	"time"
)

// init registers this plugin.
func init() { plugin.Register("spfmacro", setup) }

// setup is the function that gets called when the config parser see the token "spfmacro". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "spfmacro".
func setup(c *caddy.Controller) error {
	c.Next() // Ignore "spfmacro" and give us the next token.

	allowedSPFDomains := c.RemainingArgs()
	if len(allowedSPFDomains) == 0 {
		// If there was another token, return an error, because we don't have any configuration.
		// Any errors returned from this setup function should be wrapped with plugin.Error, so we
		// can present a slightly nicer error message to the user.
		return plugin.Error("spfmacro", errors.New("you must have at least one domain specified"))
	}

	if !CheckDomainValidity(allowedSPFDomains) {
		return plugin.Error("spfmacro", errors.New("domains must be in the form txt:example.com"))
	}
	fmt.Println(allowedSPFDomains)
	spfMacro := SPFMacro{
		Domains: allowedSPFDomains,
		M:       sync.RWMutex{},
		L:       rate.NewLimiter(rate.Every(60*time.Second), 1),
	}

	go func(spfMacro *SPFMacro) {
		for {
			if spfMacro.L.Allow() {
				ps, err := RetrieveSPFIPs(spfMacro.Domains)
				if err != nil {
					log.Errorf("There was an error retrieving SPF IPs:%s\n", err.Error())
					continue
				}

				spfMacro.M.Lock()
				spfMacro.Subnets = ps
				spfMacro.M.Unlock()
			}
		}
	}(&spfMacro)

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		spfMacro.Next = next
		return &spfMacro
	})

	// All OK, return a nil error.
	return nil
}

func CheckDomainValidity(domains []string) bool {
	for _, domain := range domains {
		if !strings.Contains(domain, ":") {
			return false
		}
	}
	return true
}
