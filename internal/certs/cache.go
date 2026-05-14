package certs

import (
	"crypto/tls"
	"sync"
	"time"
)

// CertCache implements goproxy.CertStorage to cache dynamically-signed
// leaf certificates. Without a CertStore, goproxy re-generates a new
// RSA key pair and signs a new leaf certificate on every CONNECT —
// expensive and unnecessary for repeated connections to the same host.
type CertCache struct {
	mu    sync.RWMutex
	store map[string]*cacheEntry
}

type cacheEntry struct {
	cert *tls.Certificate
	exp  time.Time
}

const certTTL = 1 * time.Hour

func NewCertCache() *CertCache {
	cc := &CertCache{store: make(map[string]*cacheEntry)}
	go cc.sweep()
	return cc
}

// Fetch returns a cached leaf cert for hostname, or calls gen to create one.
func (cc *CertCache) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	cc.mu.RLock()
	if e, ok := cc.store[hostname]; ok && time.Now().Before(e.exp) {
		cc.mu.RUnlock()
		return e.cert, nil
	}
	cc.mu.RUnlock()

	cert, err := gen()
	if err != nil {
		return nil, err
	}

	cc.mu.Lock()
	cc.store[hostname] = &cacheEntry{cert: cert, exp: time.Now().Add(certTTL)}
	cc.mu.Unlock()
	return cert, nil
}

func (cc *CertCache) sweep() {
	for range time.Tick(10 * time.Minute) {
		cc.mu.Lock()
		now := time.Now()
		for k, e := range cc.store {
			if now.After(e.exp) {
				delete(cc.store, k)
			}
		}
		cc.mu.Unlock()
	}
}