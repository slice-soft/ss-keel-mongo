package mongo

import (
	"testing"
	"time"

	"github.com/slice-soft/ss-keel-core/contracts"
)

// Compile-time interface assertions — redundant at runtime but caught by the
// compiler, mirroring the var block in addon.go.
var (
	_ contracts.Addon        = (*Client)(nil)
	_ contracts.Debuggable   = (*Client)(nil)
	_ contracts.Manifestable = (*Client)(nil)
)

func newAddonTestClient() *Client {
	return &Client{events: make(chan contracts.PanelEvent, 256)}
}

// --- ID, PanelID, PanelLabel ---

func TestAddon_ID(t *testing.T) {
	c := newAddonTestClient()
	if got := c.ID(); got != "mongo" {
		t.Fatalf("ID() = %q, want %q", got, "mongo")
	}
}

func TestAddon_PanelID(t *testing.T) {
	c := newAddonTestClient()
	if got := c.PanelID(); got != "mongo" {
		t.Fatalf("PanelID() = %q, want %q", got, "mongo")
	}
}

func TestAddon_PanelLabel(t *testing.T) {
	c := newAddonTestClient()
	if got := c.PanelLabel(); got != "Database (MongoDB)" {
		t.Fatalf("PanelLabel() = %q, want %q", got, "Database (MongoDB)")
	}
}

// --- Manifest ---

func TestAddon_Manifest(t *testing.T) {
	c := newAddonTestClient()
	m := c.Manifest()

	if m.ID != "mongo" {
		t.Errorf("Manifest.ID = %q, want %q", m.ID, "mongo")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Manifest.Version = %q, want %q", m.Version, "1.0.0")
	}
	if len(m.Capabilities) != 1 || m.Capabilities[0] != "database" {
		t.Errorf("Manifest.Capabilities = %v, want [database]", m.Capabilities)
	}
	if len(m.Resources) != 1 || m.Resources[0] != "mongodb" {
		t.Errorf("Manifest.Resources = %v, want [mongodb]", m.Resources)
	}
	if len(m.EnvVars) != 2 {
		t.Fatalf("Manifest.EnvVars length = %d, want 2", len(m.EnvVars))
	}

	uri := m.EnvVars[0]
	if uri.Key != "MONGO_URI" {
		t.Errorf("EnvVars[0].Key = %q, want %q", uri.Key, "MONGO_URI")
	}
	if !uri.Required {
		t.Error("EnvVars[0].Required should be true")
	}
	if !uri.Secret {
		t.Error("EnvVars[0].Secret should be true")
	}

	db := m.EnvVars[1]
	if db.Key != "MONGO_DATABASE" {
		t.Errorf("EnvVars[1].Key = %q, want %q", db.Key, "MONGO_DATABASE")
	}
	if !db.Required {
		t.Error("EnvVars[1].Required should be true")
	}
	if db.Secret {
		t.Error("EnvVars[1].Secret should be false")
	}
}

// --- PanelEvents ---

func TestAddon_PanelEvents_ReturnsChannel(t *testing.T) {
	c := newAddonTestClient()
	ch := c.PanelEvents()
	if ch == nil {
		t.Fatal("PanelEvents() returned nil channel")
	}
}

func TestAddon_PanelEvents_ReceivesEmittedEvent(t *testing.T) {
	c := newAddonTestClient()
	ch := c.PanelEvents()

	want := contracts.PanelEvent{
		Timestamp: time.Now(),
		AddonID:   "mongo",
		Label:     "find",
		Level:     "info",
		Detail:    map[string]any{"command": "find", "duration_ms": int64(5)},
	}
	c.tryEmit(want)

	select {
	case got := <-ch:
		if got.AddonID != want.AddonID {
			t.Errorf("AddonID = %q, want %q", got.AddonID, want.AddonID)
		}
		if got.Label != want.Label {
			t.Errorf("Label = %q, want %q", got.Label, want.Label)
		}
		if got.Level != want.Level {
			t.Errorf("Level = %q, want %q", got.Level, want.Level)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event on channel")
	}
}

// --- tryEmit non-blocking when full ---

func TestAddon_TryEmit_NonBlockingWhenFull(t *testing.T) {
	c := newAddonTestClient() // buffer = 256

	evt := contracts.PanelEvent{
		Timestamp: time.Now(),
		AddonID:   "mongo",
		Label:     "insert",
		Level:     "info",
	}

	// Send 300 events; the first 256 fill the buffer, the remaining 44 must be
	// dropped without blocking.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 300; i++ {
			c.tryEmit(evt)
		}
		close(done)
	}()

	select {
	case <-done:
		// success — no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("tryEmit blocked when channel was full")
	}
}
