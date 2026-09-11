package dav

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrCertUntrusted is returned when TLS chain verification fails and no
// accepted leaf fingerprint matches.
var ErrCertUntrusted = errors.New("dav: certificate not trusted")

// LeafFingerprint returns the lowercase hex SHA-256 of a certificate DER (F-13).
func LeafFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// NormalizeFingerprint lowercases and strips non-hex characters from input.
func NormalizeFingerprint(fp string) string {
	var b strings.Builder
	b.Grow(len(fp))
	for _, r := range strings.ToLower(fp) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func tlsConfigWithPin(serverName, pinnedFingerprint string) *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}
	pin := NormalizeFingerprint(pinnedFingerprint)
	if pin == "" {
		return cfg
	}
	// Chain verification is replaced by the accepted fingerprint for this
	// connection only (F-13); address checks stay via ServerName and Guard.
	cfg.InsecureSkipVerify = true
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return errors.New("dav: no peer certificate")
		}
		if LeafFingerprint(cs.PeerCertificates[0]) != pin {
			return fmt.Errorf("%w: fingerprint mismatch", ErrCertUntrusted)
		}
		return nil
	}
	return cfg
}
