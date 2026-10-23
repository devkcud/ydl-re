package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewConfigIsSparseAndPure(t *testing.T) {
	c := testConfig(t)
	if c.Version != 1 || c.InstanceID != testID(1) || len(c.Defaults) != 0 || len(c.Profiles) != 1 ||
		c.Profiles[0].ID != testID(2) || c.Profiles[0].Name != "Default" || len(c.Profiles[0].Settings) != 0 {
		t.Fatal("initial config did not preserve identity and inheritance")
	}

	if _, err := os.Stat(c.StateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("constructor touched state: %v", err)
	}

	got, err := New("invalid", testID(2), c.StateDir)
	if err != ErrInvalid || !reflect.DeepEqual(got, Config{}) {
		t.Fatal("invalid construction returned partial config")
	}
}

func TestConfigRejectsInvalidModel(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Config)
	}{
		{"version", func(c *Config) { c.Version++ }},
		{"instance case", func(c *Config) { c.InstanceID = strings.Repeat("A", 32) }},
		{"instance length", func(c *Config) { c.InstanceID += "0" }},
		{"relative state", func(c *Config) { c.StateDir = "relative" }},
		{"unclean state", func(c *Config) { c.StateDir += "/../state" }},
		{"state NUL", func(c *Config) { c.StateDir += "\x00" }},
		{"state Unicode", func(c *Config) { c.StateDir += "\xff" }},
		{"derived path size", func(c *Config) { c.StateDir = string(filepath.Separator) + strings.Repeat("x", MaxPathSize-1) }},
		{"no profiles", func(c *Config) { c.Profiles = nil }},
		{"duplicate ID", func(c *Config) { c.Profiles = append(c.Profiles, Profile{ID: testID(2), Name: "Pato"}) }},
		{"duplicate name", func(c *Config) { c.Profiles = append(c.Profiles, Profile{ID: testID(3), Name: "Default"}) }},
		{"profile ID", func(c *Config) { c.Profiles[0].ID = strings.Repeat("z", 32) }},
		{"empty name", func(c *Config) { c.Profiles[0].Name = "" }},
		{"name spacing", func(c *Config) { c.Profiles[0].Name = " Pato" }},
		{"name control", func(c *Config) { c.Profiles[0].Name = "Pa\nto" }},
		{"name Unicode", func(c *Config) { c.Profiles[0].Name = "\xff" }},
		{"name bytes", func(c *Config) { c.Profiles[0].Name = strings.Repeat("é", 41) }},
		{"name selector ambiguity", func(c *Config) { c.Profiles[0].Name = testID(3) }},
		{"global key", func(c *Config) { c.Defaults = Overrides{"download.url": Text("private")} }},
		{"global type", func(c *Config) { c.Defaults = Overrides{Connections: Text("4")} }},
		{"profile key", func(c *Config) { c.Profiles[0].Settings = Overrides{"unknown": Int(1)} }},
		{"profile value", func(c *Config) { c.Profiles[0].Settings = Overrides{Connections: Int(0)} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := testConfig(t)
			test.edit(&c)
			if c.Validate() != ErrInvalid {
				t.Fatal("invalid model accepted")
			}

			if got, err := c.Profile("Default"); err != ErrInvalid || !reflect.DeepEqual(got, Profile{}) {
				t.Fatal("invalid model exposed a profile")
			}
		})
	}
}

func TestConfigProfileBoundsAndExactLookup(t *testing.T) {
	c := testConfig(t)
	c.Profiles[0].Name = strings.Repeat("é", 40)
	for i := 1; i < MaxProfiles; i++ {
		c.Profiles = append(c.Profiles, Profile{ID: testID(i + 2), Name: fmt.Sprintf("Profile %d", i)})
	}
	c.Profiles[1].Name = "Pato"
	c.Profiles[2].Name = "pato"
	if err := c.Validate(); err != nil {
		t.Fatal("rejected profile/name boundary or case-sensitive names", err)
	}

	for _, selector := range []string{"Pato", c.Profiles[1].ID} {
		p, err := c.Profile(selector)
		if err != nil || p.ID != c.Profiles[1].ID {
			t.Fatal("exact selector resolved incorrectly", err)
		}
	}
	for _, selector := range []string{"", "Pat", "PATO", " Pato", testID(99), c.Profiles[1].ID[:31]} {
		p, err := c.Profile(selector)
		if err != ErrProfile || !reflect.DeepEqual(p, Profile{}) {
			t.Fatal("unknown selector exposed a profile")
		}
	}

	c.Profiles = append(c.Profiles, Profile{ID: testID(99), Name: "Extra"})
	if c.Validate() != ErrInvalid {
		t.Fatal("accepted excess profiles")
	}
}

func TestConfigCopiesOwnAllOverrides(t *testing.T) {
	c := testConfig(t)
	c.Defaults = Overrides{Retries: Int(0)}
	c.Profiles[0].Settings = Overrides{Limit: Text(Unlimited)}
	next := c.Clone()
	p, err := c.Profile("Default")
	if err != nil {
		t.Fatal(err)
	}

	next.Defaults[Retries] = Int(3)
	next.Profiles[0].Settings[Limit] = Text("1B/s")
	next.Profiles[0].Name = "Changed"
	p.Settings[Limit] = Text("2B/s")
	if c.Defaults[Retries] != Int(0) || c.Profiles[0].Settings[Limit] != Text(Unlimited) || c.Profiles[0].Name != "Default" {
		t.Fatal("copy or lookup aliased its source")
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	c, err := New(testID(1), testID(2), filepath.Join(t.TempDir(), "not-created"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testID(n int) string {
	return fmt.Sprintf("%032x", n)
}
