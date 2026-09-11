package dav

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func selfSignedTLSServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "dav.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"127.0.0.1"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	fp := LeafFingerprint(cert)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  key,
		}},
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, fp
}

func TestCertPinAcceptsSelfSignedLeaf(t *testing.T) {
	srv, fp := selfSignedTLSServer(t)
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	g := NewGuard(c)

	client := g.HTTPClient()
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected chain verification failure without pin")
	}

	pinned := g.HTTPClientWithCertPin(fp)
	resp, err := pinned.Get(srv.URL)
	if err != nil {
		t.Fatalf("pinned client: %v", err)
	}
	resp.Body.Close()
}

func TestCertPinRejectsWrongFingerprint(t *testing.T) {
	srv, _ := selfSignedTLSServer(t)
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	g := NewGuard(c)

	wrong := stringsRepeat("ab", 32)
	pinned := g.HTTPClientWithCertPin(wrong)
	_, err := pinned.Get(srv.URL)
	if err == nil {
		t.Fatal("expected pin mismatch error")
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}

func TestNormalizeFingerprint(t *testing.T) {
	got := NormalizeFingerprint("AB:CD-ef 12")
	want := "abcdef12"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRedirectRefusesHTTPSDowngrade(t *testing.T) {
	httpHits := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(httpSrv.Close)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "dav.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	tlsSrv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpSrv.URL, http.StatusFound)
	}))
	tlsSrv.TLS = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  key,
		}},
	}
	tlsSrv.StartTLS()
	t.Cleanup(tlsSrv.Close)

	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	client := NewGuard(c).HTTPClientWithCertPin(LeafFingerprint(cert))
	_, err = client.Get(tlsSrv.URL)
	if err == nil {
		t.Fatal("expected HTTPS→HTTP redirect to be refused")
	}
	if !strings.Contains(err.Error(), "HTTPS") && !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("error = %v, want downgrade refusal", err)
	}
	if httpHits != 0 {
		t.Fatalf("downgraded request reached HTTP server (%d hits)", httpHits)
	}
}
