// Package cert serves a TLS certificate from disk and reloads it when the files
// change on renewal (acme.sh rewrites them in place), without a restart.
package cert

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"
)

// Reloader serves the TLS cert from disk and reloads it when the cert file's
// mtime advances (at most once a minute).
type Reloader struct {
	certPath string
	keyPath  string

	mu       sync.RWMutex
	cert     *tls.Certificate
	loadedAt time.Time
	certMod  time.Time
}

// NewReloader loads the initial cert.
func NewReloader(certPath, keyPath string) (*Reloader, error) {
	c := &Reloader{certPath: certPath, keyPath: keyPath}
	if err := c.reload(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Reloader) reload() error {
	cert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	mod := time.Time{}
	if fi, err := os.Stat(c.certPath); err == nil {
		mod = fi.ModTime()
	}

	c.mu.Lock()
	c.cert = &cert
	c.loadedAt = time.Now()
	c.certMod = mod
	c.mu.Unlock()

	return nil
}

// GetCertificate is the tls.Config hook. It reloads at most once a minute when the
// cert file's mtime advances.
func (c *Reloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.RLock()
	cert, loadedAt, certMod := c.cert, c.loadedAt, c.certMod
	c.mu.RUnlock()

	if time.Since(loadedAt) > time.Minute {
		if fi, err := os.Stat(c.certPath); err == nil && fi.ModTime().After(certMod) {
			if err := c.reload(); err == nil {
				c.mu.RLock()
				cert = c.cert
				c.mu.RUnlock()
			}
		} else {
			// Touch loadedAt so we don't stat on every handshake.
			c.mu.Lock()
			c.loadedAt = time.Now()
			c.mu.Unlock()
		}
	}

	return cert, nil
}
