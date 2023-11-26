package spfmacro

import (
	"bytes"
	"context"
	"fmt"
	golog "log"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestSPFMacro(t *testing.T) {
	// Create a new SPFMacro Plugin. Use the test.ErrorHandler as the next plugin.
	x := SPFMacro{Next: test.ErrorHandler()}

	// Setup a new output buffer that is *not* standard output, so we can check if
	// example is really being printed.
	b := &bytes.Buffer{}
	golog.SetOutput(b)

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)
	// Create a new Recorder that captures the result, this isn't actually used in this test
	// as it just serves as something that implements the dns.ResponseWriter interface.
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	// Call our plugin directly, and check the result.
	x.ServeDNS(ctx, rec, r)
	a := b.String()
	if !strings.Contains(a, "[INFO] plugin/spfmacro: spfmacro") {
		t.Errorf("Failed to print '%s', got %s", "[INFO] plugin/example: example", a)
	}
}

func TestIPParsing(t *testing.T) {
	ps, err := RetrieveSPFIPs([]string{"include:_despf.mail.wku.edu"})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(ps)

}
