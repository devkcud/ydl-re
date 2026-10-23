package config

import (
	"encoding/hex"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Version = 1
const MaxProfiles = 32
const MaxNameSize = 80
const MaxPathSize = 4096

var ErrInvalid = errors.New("invalid configuration")
var ErrProfile = errors.New("configuration profile not found")

type Overrides map[Key]Value

type Profile struct {
	ID       string
	Name     string
	Settings Overrides
}

type Config struct {
	Version    int
	InstanceID string
	StateDir   string
	Defaults   Overrides
	Profiles   []Profile
}

func New(id, profile, dir string) (Config, error) {
	c := Config{
		Version: Version, InstanceID: id, StateDir: dir,
		Profiles: []Profile{{ID: profile, Name: "Default"}},
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}

	return c, nil
}

func (c Config) Validate() error {
	if c.Version != Version || !validID(c.InstanceID) || !validPath(c.StateDir) ||
		!validPath(filepath.Join(c.StateDir, "transfers")) || !c.Defaults.valid() ||
		len(c.Profiles) == 0 || len(c.Profiles) > MaxProfiles {
		return ErrInvalid
	}

	ids := make(map[string]bool, len(c.Profiles))
	names := make(map[string]bool, len(c.Profiles))
	for _, p := range c.Profiles {
		if !validID(p.ID) || !validName(p.Name) || !p.Settings.valid() || ids[p.ID] || names[p.Name] {
			return ErrInvalid
		}

		ids[p.ID] = true
		names[p.Name] = true
	}

	return nil
}

func (c Config) Clone() Config {
	c.Defaults = maps.Clone(c.Defaults)
	c.Profiles = slices.Clone(c.Profiles)
	for i := range c.Profiles {
		c.Profiles[i].Settings = maps.Clone(c.Profiles[i].Settings)
	}

	return c
}

func (c Config) Profile(selector string) (Profile, error) {
	if err := c.Validate(); err != nil {
		return Profile{}, err
	}

	i, err := c.profileIndex(selector)
	if err != nil {
		return Profile{}, err
	}

	p := c.Profiles[i]
	p.Settings = maps.Clone(p.Settings)
	return p, nil
}

func (c Config) profileIndex(selector string) (int, error) {
	for i, p := range c.Profiles {
		if p.ID == selector || p.Name == selector {
			return i, nil
		}
	}

	return 0, ErrProfile
}

func (o Overrides) valid() bool {
	for key, value := range o {
		if key.validate(value) != nil {
			return false
		}
	}

	return true
}

func validID(id string) bool {
	if len(id) != 32 || strings.ToLower(id) != id {
		return false
	}

	_, err := hex.DecodeString(id)
	return err == nil
}

func validName(name string) bool {
	return name != "" && len(name) <= MaxNameSize && utf8.ValidString(name) &&
		strings.TrimSpace(name) == name && strings.IndexFunc(name, unicode.IsControl) < 0 && !validID(name)
}

func validPath(path string) bool {
	return len(path) <= MaxPathSize && utf8.ValidString(path) && !strings.ContainsRune(path, 0) &&
		filepath.IsAbs(path) && filepath.Clean(path) == path
}
