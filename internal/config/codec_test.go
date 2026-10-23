package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestConfigCodecCanonicalFormat(t *testing.T) {
	c := testConfig(t)
	want := fmt.Sprintf(`{
  "version": 1,
  "instance_id": "%s",
  "state_dir": %s,
  "defaults": {},
  "profiles": [
    {
      "id": "%s",
      "name": "Default",
      "settings": {}
    }
  ]
}
`, c.InstanceID, configJSONBytes(t, c.StateDir), c.Profiles[0].ID)
	data := encodedConfig(t, c)
	if string(data) != want {
		t.Fatal("canonical field order, indentation, empty objects, or newline changed")
	}
	got, err := decodeConfig(data)
	if err != nil || !reflect.DeepEqual(got, c) || !bytes.Equal(encodedConfig(t, got), data) {
		t.Fatal("initial config failed canonical round trip", err)
	}

	c.Defaults = Overrides{}
	c.Profiles[0].Settings = Overrides{}
	if !bytes.Equal(encodedConfig(t, c), data) {
		t.Fatal("empty and absent override maps encoded differently")
	}
	root, fields := configFields(t, data, "defaults")
	fields["download"] = map[string]any{}
	fields["storage"] = map[string]any{}
	got, err = decodeConfig(configJSONBytes(t, root))
	if err != nil || got.Defaults != nil || !bytes.Equal(encodedConfig(t, got), data) {
		t.Fatal("empty groups did not normalize to inheritance", err)
	}
}

func TestConfigCodecRoundTripPreservesTypesAndOwnership(t *testing.T) {
	c := codecConfig(t)
	before := c.Clone()
	data := encodedConfig(t, c)
	if !reflect.DeepEqual(c, before) {
		t.Fatal("encoding modified its input")
	}
	a, err := decodeConfig(data)
	if err != nil || !reflect.DeepEqual(a, c) {
		t.Fatal("round trip changed sparse settings or profile order", err)
	}
	b, err := decodeConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	a.Defaults[Connections] = Int(1)
	a.Profiles[0].Settings[Retries] = Int(1)
	clear(data)
	if !reflect.DeepEqual(b, c) || !reflect.DeepEqual(c, before) {
		t.Fatal("decoded config aliased another config or its input bytes")
	}
	r, err := b.Resolve(Scope{Profile: "Default"})
	if err != nil || r.Settings.TimeoutNS != math.MaxInt64 || r.Settings.LimitBPS != math.MaxInt64 || r.Settings.Retries != 0 {
		t.Fatal("codec lost integer precision or explicit zero", err)
	}

	b.Profiles[0].Settings[Limit] = Text(Unlimited)
	got, err := decodeConfig(encodedConfig(t, b))
	if err != nil || got.Profiles[0].Settings[Limit] != Text(Unlimited) {
		t.Fatal("explicit unlimited became absence", err)
	}
}

func TestConfigCodecBounds(t *testing.T) {
	c := testConfig(t)
	data := encodedConfig(t, c)
	padded := append(data, bytes.Repeat([]byte{' '}, MaxConfigSize-len(data))...)
	if _, err := decodeConfig(padded); err != nil {
		t.Fatal("rejected exact raw byte limit", err)
	}
	refuseConfigJSON(t, append(padded, ' '))

	c.Defaults = Overrides{Timeout: Text("1s")}
	base := encodedConfig(t, c)
	c.Defaults[Timeout] = Text(strings.Repeat("0", MaxConfigSize-len(base)) + "1s")
	if data := encodedConfig(t, c); len(data) != MaxConfigSize {
		t.Fatal("canonical boundary fixture has the wrong size")
	} else if _, err := decodeConfig(data); err != nil {
		t.Fatal("rejected exact canonical byte limit", err)
	}
	c.Defaults[Timeout] = Text("0" + c.Defaults[Timeout].text)
	if c.Validate() != nil {
		t.Fatal("size fixture must be structurally valid")
	}
	refuseConfigEncoding(t, c)
	c.Defaults[Timeout] = Text(strings.Repeat("0", MaxConfigSize*2) + "1s")
	refuseConfigEncoding(t, c)

	c = testConfig(t)
	for i := 1; i < MaxProfiles; i++ {
		c.Profiles = append(c.Profiles, Profile{ID: testID(i + 2), Name: fmt.Sprintf("Profile %d", i)})
	}
	if _, err := decodeConfig(encodedConfig(t, c)); err != nil {
		t.Fatal("rejected maximum profile count", err)
	}
	root, _ := configFields(t, encodedConfig(t, c), "root")
	root["profiles"] = append(root["profiles"].([]any), map[string]any{"id": testID(99), "name": "Extra", "settings": map[string]any{}})
	refuseConfigJSON(t, configJSONBytes(t, root))
	c.Profiles = append(c.Profiles, Profile{ID: testID(99), Name: "Extra"})
	refuseConfigEncoding(t, c)
}

func TestConfigCodecRejectsCanonicalExpansion(t *testing.T) {
	c := testConfig(t)
	dir := filepath.Join(c.StateDir, strings.Repeat("<", 1800))
	profiles := make([]any, 0, MaxProfiles)
	c.Profiles = nil
	for i := 0; i < MaxProfiles; i++ {
		p := Profile{ID: testID(i + 2), Name: fmt.Sprintf("Profile %d", i), Settings: Overrides{OutputDir: Text(dir)}}
		c.Profiles = append(c.Profiles, p)
		profiles = append(profiles, map[string]any{
			"id": p.ID, "name": p.Name, "settings": map[string]any{"download": map[string]any{"output_dir": dir}},
		})
	}
	if c.Validate() != nil {
		t.Fatal("expansion fixture must have valid fields")
	}
	refuseConfigEncoding(t, c)
	raw := configJSONBytes(t, map[string]any{
		"version": 1, "instance_id": c.InstanceID, "state_dir": c.StateDir,
		"defaults": map[string]any{}, "profiles": profiles,
	})
	raw = bytes.ReplaceAll(raw, []byte(`\u003c`), []byte("<"))
	if len(raw) >= MaxConfigSize {
		t.Fatal("compact fixture must fit the input limit")
	}
	refuseConfigJSON(t, raw)
}

func TestConfigCodecRejectsInvalidModelsWithoutMutation(t *testing.T) {
	for _, edit := range []func(*Config){
		func(c *Config) { c.Version++ },
		func(c *Config) { c.InstanceID = "invalid" },
		func(c *Config) { c.StateDir = "relative" },
		func(c *Config) { c.Profiles = nil },
		func(c *Config) { c.Profiles[0].Name = "\xff" },
		func(c *Config) { c.Defaults = Overrides{"private-invalid": Text("private-invalid")} },
		func(c *Config) { c.Profiles[0].Settings = Overrides{Retries: Text("0")} },
	} {
		c := testConfig(t)
		edit(&c)
		before := c.Clone()
		refuseConfigEncoding(t, c)
		if !reflect.DeepEqual(c, before) {
			t.Fatal("failed encoding modified its input")
		}
	}
}

func FuzzConfigCodec(f *testing.F) {
	c, err := New(testID(1), testID(2), filepath.Join(f.TempDir(), "state"))
	if err != nil {
		f.Fatal(err)
	}
	for _, defaults := range []Overrides{nil, {Retries: Int(0), Limit: Text(Unlimited), Timeout: Text("1m30s")}} {
		c.Defaults = defaults
		data, err := encodeConfig(c)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
		f.Add(append([]byte(" \n"), data...))
	}
	f.Add([]byte(`null`))
	f.Add([]byte(`{"version":1,"version":1}`))
	f.Add([]byte(`"\ud800"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, token, err := Decode(data)
		if err != nil {
			if err != ErrInvalid || token != "" || !reflect.DeepEqual(c, Config{}) {
				t.Fatal("invalid input exposed partial config or parser diagnostics")
			}
			return
		}
		if token != fmt.Sprintf("%x", sha256.Sum256(data)) {
			t.Fatal("token does not match validated input bytes")
		}
		encoded, canonical, err := Encode(c)
		if err != nil || canonical != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("valid config failed encoding or token generation", err)
		}
		got, token, err := Decode(encoded)
		if err != nil || token != canonical || !reflect.DeepEqual(got, c) || !bytes.Equal(encodedConfig(t, got), encoded) {
			t.Fatal("valid config failed canonical round trip", err)
		}
	})
}

func codecConfig(t *testing.T) Config {
	t.Helper()
	c := testConfig(t)
	c.Defaults = Overrides{
		Connections: Int(4), Retries: Int(3), Limit: Text("5MiB/s"), Timeout: Text("30s"),
		OutputDir: Text(filepath.Join(c.StateDir, "global-output")),
		StateDir:  Text(filepath.Join(c.StateDir, "global-state")), StagingDir: Text(Destination),
	}
	max := strconv.FormatInt(math.MaxInt64, 10)
	c.Profiles[0].Settings = Overrides{
		Connections: Int(8), Retries: Int(0), Timeout: Text(max + "ns"), Limit: Text(max + "B/s"),
		OutputDir: Text(filepath.Join(c.StateDir, "profile-output")),
		StateDir:  Text(filepath.Join(c.StateDir, "profile-state")), StagingDir: Text(filepath.Join(c.StateDir, "staging")),
	}
	c.Profiles = append(c.Profiles, Profile{ID: testID(3), Name: "Pato 🐈"})
	return c
}

func encodedConfig(t *testing.T, c Config) []byte {
	t.Helper()
	data, err := encodeConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func configJSONBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func refuseConfigJSON(t *testing.T, data []byte) {
	t.Helper()
	got, err := decodeConfig(data)
	if err != ErrInvalid || !reflect.DeepEqual(got, Config{}) {
		t.Fatal("invalid input exposed partial config or unsafe error", err)
	}
}

func refuseConfigEncoding(t *testing.T, c Config) {
	t.Helper()
	data, err := encodeConfig(c)
	if err != ErrInvalid || data != nil {
		t.Fatal("invalid config exposed partial output or unsafe error", err)
	}
}
