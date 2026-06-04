// Package ruleset mirrors upstream rule-provider files so clients fetch them from
// subgen (over the RU1 TLS endpoint) instead of GitHub, which can be unreachable
// from RU networks. It is the service layer for rule mirroring.
package ruleset

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/postlog/subgen/internal/mihomo"
)

type entry struct {
	data        []byte
	contentType string
}

// Mirror holds the latest copy of each mirrored provider in memory.
type Mirror struct {
	hc   *http.Client
	mu   sync.RWMutex
	data map[string]entry // keyed by "<name><ext>"
	srcs map[string]source
}

type source struct {
	url      string
	ext      string
	interval time.Duration
}

// New builds a mirror for the providers that have Mirror == true. The provider set
// is fixed at construction (changing providers needs a restart).
func New(providers []mihomo.RuleProvider) *Mirror {
	m := &Mirror{
		hc:   &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}},
		data: map[string]entry{},
		srcs: map[string]source{},
	}

	for _, rp := range providers {
		if !rp.Mirror {
			continue
		}

		ext := extFor(rp.Format)

		// MirrorInterval drives subgen's mirror refresh; the mihomo client TTL
		// (rp.Interval) is separate. Fall back to 24h when unset.
		interval := time.Duration(rp.MirrorInterval) * time.Second
		if interval <= 0 {
			interval = 24 * time.Hour
		}

		m.srcs[rp.Name+ext] = source{url: rp.URL, ext: ext, interval: interval}
	}

	return m
}

// Get returns the cached bytes + content type for a file.
func (m *Mirror) Get(file string) ([]byte, string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.data[file]

	return e.data, e.contentType, ok
}

// Run fetches every mirrored provider once, then refreshes on each one's interval
// until ctx is cancelled. It blocks; run it in a goroutine.
func (m *Mirror) Run(ctx context.Context) {
	if len(m.srcs) == 0 {
		return
	}

	for file, src := range m.srcs {
		m.fetch(ctx, file, src) // best-effort initial load
		go m.loop(ctx, file, src)
	}

	<-ctx.Done()
}

func (m *Mirror) loop(ctx context.Context, file string, src source) {
	t := time.NewTicker(src.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.fetch(ctx, file, src)
		}
	}
}

func (m *Mirror) fetch(ctx context.Context, file string, src source) {
	data, err := m.download(ctx, src.url)
	if err != nil || len(data) == 0 {
		return // keep last good copy
	}

	m.mu.Lock()
	m.data[file] = entry{data: data, contentType: contentTypeFor(src.ext)}
	m.mu.Unlock()
}

func (m *Mirror) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.hc.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func extFor(format string) string {
	switch strings.ToLower(format) {
	case "mrs":
		return ".mrs"
	case "yaml":
		return ".yaml"
	default:
		return ".txt"
	}
}

func contentTypeFor(ext string) string {
	switch ext {
	case ".yaml":
		return "application/yaml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
