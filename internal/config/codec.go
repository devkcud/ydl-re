package config

import (
	"encoding/json"
	"strings"
)

const MaxConfigSize = 256 * 1024

type configJSON struct {
	Version    int                       `json:"version"`
	InstanceID string                    `json:"instance_id"`
	StateDir   string                    `json:"state_dir"`
	Defaults   map[string]map[string]any `json:"defaults"`
	Profiles   []profileJSON             `json:"profiles"`
}

type profileJSON struct {
	ID       string                    `json:"id"`
	Name     string                    `json:"name"`
	Settings map[string]map[string]any `json:"settings"`
}

func encodeConfig(c Config) ([]byte, error) {
	if !boundedConfig(c) || c.Validate() != nil {
		return nil, ErrInvalid
	}

	defaults, err := encodeOverrides(c.Defaults)
	if err != nil {
		return nil, err
	}
	w := configJSON{
		Version: c.Version, InstanceID: c.InstanceID, StateDir: c.StateDir,
		Defaults: defaults, Profiles: make([]profileJSON, 0, len(c.Profiles)),
	}
	for _, p := range c.Profiles {
		settings, err := encodeOverrides(p.Settings)
		if err != nil {
			return nil, err
		}
		w.Profiles = append(w.Profiles, profileJSON{ID: p.ID, Name: p.Name, Settings: settings})
	}

	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil || len(data) >= MaxConfigSize {
		return nil, ErrInvalid
	}
	return append(data, '\n'), nil
}

func encodeOverrides(values Overrides) (map[string]map[string]any, error) {
	groups := make(map[string]map[string]any)
	for key, value := range values {
		group, name, found := strings.Cut(string(key), ".")
		if !found || group != "download" && group != "storage" {
			return nil, ErrInvalid
		}
		if groups[group] == nil {
			groups[group] = make(map[string]any)
		}
		switch value.kind {
		case intValue:
			groups[group][name] = value.num
		case textValue:
			groups[group][name] = value.text
		default:
			return nil, ErrInvalid
		}
	}
	return groups, nil
}

func boundedConfig(c Config) bool {
	if len(c.Profiles) > MaxProfiles {
		return false
	}
	left := MaxConfigSize
	take := func(s string) bool {
		if len(s) > left {
			return false
		}
		left -= len(s)
		return true
	}
	values := func(o Overrides) bool {
		if len(o) > len(keys) {
			return false
		}
		for key, value := range o {
			if !take(string(key)) || !take(value.text) {
				return false
			}
		}
		return true
	}
	if !take(c.InstanceID) || !take(c.StateDir) || !values(c.Defaults) {
		return false
	}
	for _, p := range c.Profiles {
		if !take(p.ID) || !take(p.Name) || !values(p.Settings) {
			return false
		}
	}
	return true
}
