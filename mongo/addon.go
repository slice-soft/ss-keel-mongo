package mongo

import "github.com/slice-soft/ss-keel-core/contracts"

var (
	_ contracts.Addon        = (*Client)(nil)
	_ contracts.Debuggable   = (*Client)(nil)
	_ contracts.Manifestable = (*Client)(nil)
)

// ID implements contracts.Addon.
func (c *Client) ID() string { return "mongo" }

// PanelID implements contracts.Debuggable.
func (c *Client) PanelID() string { return "mongo" }

// PanelLabel implements contracts.Debuggable.
func (c *Client) PanelLabel() string { return "Database (MongoDB)" }

// PanelEvents implements contracts.Debuggable.
func (c *Client) PanelEvents() <-chan contracts.PanelEvent { return c.events }

// Manifest implements contracts.Manifestable.
func (c *Client) Manifest() contracts.AddonManifest {
	return contracts.AddonManifest{
		ID:           "mongo",
		Version:      "1.0.0",
		Capabilities: []string{"database"},
		Resources:    []string{"mongodb"},
		EnvVars: []contracts.EnvVar{
			{Key: "MONGO_URI", Description: "MongoDB connection URI", Required: true, Secret: true, Source: "mongo"},
			{Key: "MONGO_DATABASE", Description: "MongoDB database name to connect to", Required: true, Secret: false, Source: "mongo"},
		},
	}
}

// RegisterWithPanel registers this client as a debuggable addon with a PanelRegistry.
func (c *Client) RegisterWithPanel(r contracts.PanelRegistry) {
	r.RegisterAddon(c)
}
