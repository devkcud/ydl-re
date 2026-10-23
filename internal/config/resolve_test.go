package config

import (
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultsNeedNoConfigOrOutputDirectory(t *testing.T) {
	c := testConfig(t)
	r, err := Defaults(c.StateDir)
	want := Settings{
		Connections: 4, Retries: 3, TimeoutNS: 30000000000,
		StateDir: filepath.Join(c.StateDir, "transfers"), StagingDir: Destination,
	}
	if err != nil || r.Settings != want || r.ProfileID != "" || r.ProfileName != "" || len(r.Entries) != len(Keys()) {
		t.Fatal("incorrect built-in settings", r, err)
	}
	for i, entry := range r.Entries {
		if entry.Key != Keys()[i] || entry.Source != SourceBuiltin {
			t.Fatal("built-in provenance or order changed")
		}
		if entry.Key == OutputDir && entry.Value != (Value{}) {
			t.Fatal("unconfigured output became explicit empty text")
		}
	}

	for _, dir := range []string{"", "relative", c.StateDir + "/../state", c.StateDir + "\x00", c.StateDir + "\xff", c.StateDir + strings.Repeat("x", MaxPathSize)} {
		got, err := Defaults(dir)
		if err != ErrInvalid || !reflect.DeepEqual(got, Resolution{}) {
			t.Fatal("invalid default state returned partial settings")
		}
	}
}

func TestResolutionPreservesPrecedenceOriginsAndExplicitZero(t *testing.T) {
	c := testConfig(t)
	out := filepath.Join(c.StateDir, "downloads")
	state := filepath.Join(c.StateDir, "profile-state")
	c.Defaults = Overrides{
		Connections: Int(8), Retries: Int(5), Limit: Text("5MB/s"),
		Timeout: Text("1m30s"), OutputDir: Text(out),
	}
	c.Profiles = append(c.Profiles, Profile{
		ID: testID(3), Name: "Pato",
		Settings: Overrides{Connections: Int(16), Retries: Int(0), Limit: Text(Unlimited), StateDir: Text(state)},
	})

	global, err := c.Resolve(Scope{Global: true})
	if err != nil || global.ProfileID != "" || global.Settings.Connections != 8 || global.Settings.LimitBPS != 5000000 {
		t.Fatal("global defaults resolved incorrectly", err)
	}
	for _, selector := range []string{"Pato", testID(3)} {
		r, err := c.Resolve(Scope{Profile: selector})
		want := Settings{
			OutputDir: out, Connections: 16, Retries: 0, TimeoutNS: 90000000000,
			LimitBPS: 0, StateDir: state, StagingDir: Destination,
		}
		if err != nil || r.Settings != want || r.ProfileID != testID(3) || r.ProfileName != "Pato" {
			t.Fatal("profile resolution lost inheritance or explicit zero", r, err)
		}
		wantSources := []Source{SourceProfile, SourceProfile, SourceGlobal, SourceProfile, SourceGlobal, SourceBuiltin, SourceProfile}
		for i, entry := range r.Entries {
			if entry.Key != Keys()[i] || entry.Source != wantSources[i] {
				t.Fatal("effective setting has incorrect origin or order", entry)
			}
		}
	}

	other, err := c.Resolve(Scope{Profile: "Default"})
	if err != nil || other.Settings != global.Settings || other.ProfileID != testID(2) {
		t.Fatal("another profile inherited the selected profile's overrides", err)
	}
}

func TestScopedReadsDoNotConfuseUnsetWithInheritance(t *testing.T) {
	c := testConfig(t)
	c.Defaults = Overrides{Timeout: Text("1m"), Connections: Int(8)}
	scope := Scope{Profile: "Default"}
	if value, err := c.Get(scope, Connections); err != ErrUnset || value != (Value{}) {
		t.Fatal("raw profile read returned an inherited value")
	}
	entries, err := c.List(scope)
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatal("empty scope did not return an owned empty list")
	}
	if value, err := c.Get(Scope{Global: true}, Connections); err != nil || value != Int(8) {
		t.Fatal("explicit global value was unavailable")
	}
	entries, err = c.List(Scope{Global: true})
	want := []Entry{{Connections, Int(8), SourceGlobal}, {Timeout, Text("1m"), SourceGlobal}}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatal("raw list lost scope or ordering", entries, err)
	}
	entries[0].Value = Int(1)
	if c.Defaults[Connections] != Int(8) {
		t.Fatal("raw list aliased configuration")
	}
	if value, err := c.Get(scope, "unknown"); err != ErrKey || value != (Value{}) {
		t.Fatal("unknown key was treated as an unset setting")
	}
}

func TestReadsRejectInvalidScopeAndConfigWithoutPartialResults(t *testing.T) {
	c := testConfig(t)
	for _, test := range []struct {
		scope Scope
		err   error
	}{
		{Scope{}, ErrScope},
		{Scope{Global: true, Profile: "Default"}, ErrScope},
		{Scope{Profile: "Def"}, ErrProfile},
		{Scope{Profile: testID(99)}, ErrProfile},
	} {
		if got, err := c.Resolve(test.scope); err != test.err || !reflect.DeepEqual(got, Resolution{}) {
			t.Fatal("invalid scope exposed effective settings", err)
		}
		if got, err := c.List(test.scope); err != test.err || got != nil {
			t.Fatal("invalid scope exposed raw entries", err)
		}
		if got, err := c.Get(test.scope, Connections); err != test.err || got != (Value{}) {
			t.Fatal("invalid scope exposed a value", err)
		}
	}

	c.Profiles[0].Settings = Overrides{Limit: Text("private-invalid")}
	scope := Scope{Global: true}
	if got, err := c.Resolve(scope); err != ErrInvalid || !reflect.DeepEqual(got, Resolution{}) {
		t.Fatal("unselected invalid profile was ignored during resolution")
	}
	if got, err := c.List(scope); err != ErrInvalid || got != nil {
		t.Fatal("invalid configuration exposed raw entries")
	}
	if got, err := c.Get(scope, Connections); err != ErrInvalid || got != (Value{}) {
		t.Fatal("invalid configuration exposed a raw value")
	}
}

func TestResolvedSettingsRetainExactIntegersAndOwnTheirValues(t *testing.T) {
	c := testConfig(t)
	max := strconv.FormatInt(math.MaxInt64, 10)
	c.Defaults = Overrides{Timeout: Text(max + "ns"), Limit: Text(max + "B/s")}
	c.Profiles[0].Settings = Overrides{OutputDir: Text(filepath.Join(c.StateDir, "downloads")), Connections: Int(4)}
	scope := Scope{Profile: "Default"}
	r, err := c.Resolve(scope)
	if err != nil || r.Settings.TimeoutNS != math.MaxInt64 || r.Settings.LimitBPS != math.MaxInt64 {
		t.Fatal("resolution lost int64 precision", err)
	}
	saved := r.Settings
	savedEntries := append([]Entry(nil), r.Entries...)
	c.Defaults[Timeout] = Text("1s")
	c.Defaults[Limit] = Text("1MiB/s")
	c.Profiles[0].Settings[OutputDir] = Text(filepath.Join(c.StateDir, "elsewhere"))
	c.Profiles[0].Name = "Changed"
	if r.Settings != saved || !reflect.DeepEqual(r.Entries, savedEntries) || r.ProfileName != "Default" {
		t.Fatal("configuration edits changed an earlier effective snapshot")
	}

	r.Entries[0].Value = Int(32)
	r.Settings.OutputDir = "changed by reader"
	next, err := c.Resolve(Scope{Profile: testID(2)})
	if err != nil || next.Settings.LimitBPS != 1048576 || next.Settings.TimeoutNS != 1000000000 ||
		next.Settings.OutputDir != c.Profiles[0].Settings[OutputDir].text || next.Settings.Connections != 4 {
		t.Fatal("returned state aliased future resolutions or normalized incorrectly", err)
	}
}
