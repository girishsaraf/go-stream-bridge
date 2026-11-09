// Package config contains configuration shared by first-party connectors.
package config

import "errors"

// Config locates the legacy configuration files used by a connector.
type Config struct {
	ConfigDir string `json:"configDir"`
}

// Validate rejects empty configuration directories.
func (c Config) Validate() error {
	if c.ConfigDir == "" {
		return errors.New("configDir is required")
	}
	return nil
}
