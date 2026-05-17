package config

import (
	"testing"
	"time"
)

func TestNewClientOptionsDefaults(t *testing.T) {
	opts := NewClientOptions()

	if opts.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want %v", opts.Timeout, 5*time.Second)
	}
	if !opts.EnableCache {
		t.Fatal("EnableCache = false, want true")
	}
	if opts.CacheMaxEntries != defaultClientCacheMaxEntries {
		t.Fatalf("CacheMaxEntries = %d, want %d", opts.CacheMaxEntries, defaultClientCacheMaxEntries)
	}
	if opts.CacheTTL != defaultClientCacheTTL {
		t.Fatalf("CacheTTL = %v, want %v", opts.CacheTTL, defaultClientCacheTTL)
	}
	if opts.WatchMode != WatchModeOff {
		t.Fatalf("WatchMode = %d, want %d", opts.WatchMode, WatchModeOff)
	}
	if opts.WatchScope != WatchScopeGroup {
		t.Fatalf("WatchScope = %d, want %d", opts.WatchScope, WatchScopeGroup)
	}
	if opts.WatchBuffer != 8 {
		t.Fatalf("WatchBuffer = %d, want %d", opts.WatchBuffer, 8)
	}
}

func TestNewClientOptionsApplyOverrides(t *testing.T) {
	opts := NewClientOptions(
		WithClientTimeout(12*time.Second),
		WithClientCacheEnabled(false),
		WithClientCacheMaxEntries(256),
		WithClientCacheTTL(2*time.Minute),
		WithClientWatchMode(WatchModeOn),
		WithClientWatchScope(WatchScopeApp),
		WithClientWatchBuffer(32),
	)

	if opts.Timeout != 12*time.Second {
		t.Fatalf("Timeout = %v, want %v", opts.Timeout, 12*time.Second)
	}
	if opts.EnableCache {
		t.Fatal("EnableCache = true, want false")
	}
	if opts.CacheMaxEntries != 256 {
		t.Fatalf("CacheMaxEntries = %d, want %d", opts.CacheMaxEntries, 256)
	}
	if opts.CacheTTL != 2*time.Minute {
		t.Fatalf("CacheTTL = %v, want %v", opts.CacheTTL, 2*time.Minute)
	}
	if opts.WatchMode != WatchModeOn {
		t.Fatalf("WatchMode = %d, want %d", opts.WatchMode, WatchModeOn)
	}
	if opts.WatchScope != WatchScopeApp {
		t.Fatalf("WatchScope = %d, want %d", opts.WatchScope, WatchScopeApp)
	}
	if opts.WatchBuffer != 32 {
		t.Fatalf("WatchBuffer = %d, want %d", opts.WatchBuffer, 32)
	}
}

func TestNewClientOptionsIgnoreInvalidOverrides(t *testing.T) {
	opts := NewClientOptions(
		WithClientTimeout(0),
		WithClientCacheMaxEntries(0),
		WithClientCacheTTL(0),
		WithClientWatchBuffer(0),
	)

	if opts.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want %v", opts.Timeout, 5*time.Second)
	}
	if opts.CacheMaxEntries != defaultClientCacheMaxEntries {
		t.Fatalf("CacheMaxEntries = %d, want %d", opts.CacheMaxEntries, defaultClientCacheMaxEntries)
	}
	if opts.CacheTTL != defaultClientCacheTTL {
		t.Fatalf("CacheTTL = %v, want %v", opts.CacheTTL, defaultClientCacheTTL)
	}
	if opts.WatchBuffer != 8 {
		t.Fatalf("WatchBuffer = %d, want %d", opts.WatchBuffer, 8)
	}
}
