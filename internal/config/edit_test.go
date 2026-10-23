package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestScopedEditsSupportEveryKeyAndPreserveIdentity(t *testing.T) {
	c := testConfig(t)
	c.Profiles = append(c.Profiles, Profile{ID: testID(3), Name: "Other", Settings: Overrides{Limit: Text("1B/s")}})
	before := c.Clone()
	for _, scope := range []Scope{{Global: true}, {Profile: "Default"}, {Profile: testID(2)}} {
		for _, test := range []struct {
			key   Key
			value Value
		}{
			{Connections, Int(32)},
			{Limit, Text(Unlimited)},
			{OutputDir, Text(filepath.Join(c.StateDir, "output"))},
			{Retries, Int(0)},
			{Timeout, Text("2s")},
			{StagingDir, Text(Destination)},
			{StateDir, Text(filepath.Join(c.StateDir, "checkpoints"))},
		} {
			next, changed, err := c.Set(scope, test.key, test.value)
			if err != nil || !changed || !reflect.DeepEqual(c, before) {
				t.Fatal("set failed or modified its input", scope, test.key, err)
			}
			if next.InstanceID != c.InstanceID || next.StateDir != c.StateDir || next.Version != c.Version ||
				next.Profiles[0].ID != c.Profiles[0].ID || next.Profiles[0].Name != c.Profiles[0].Name ||
				!reflect.DeepEqual(next.Profiles[1], c.Profiles[1]) {
				t.Fatal("scoped edit changed identity or another profile")
			}
			if got, err := next.Get(scope, test.key); err != nil || got != test.value {
				t.Fatal("value was not stored in its requested scope", err)
			}

			same, changed, err := next.Set(scope, test.key, test.value)
			if err != nil || changed || !reflect.DeepEqual(same, next) {
				t.Fatal("identical explicit value was not a no-op", err)
			}
			cleared, changed, err := same.Unset(scope, test.key)
			if err != nil || !changed {
				t.Fatal("explicit value was not removed", err)
			}
			if _, err := cleared.Get(scope, test.key); err != ErrUnset {
				t.Fatal("unset left an explicit value")
			}
			want, err := c.Resolve(scope)
			if err != nil {
				t.Fatal(err)
			}
			got, err := cleared.Resolve(scope)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("unset did not restore inherited settings", err)
			}
			if _, changed, err := cleared.Unset(scope, test.key); err != nil || changed {
				t.Fatal("unsetting an absent value was not a no-op", err)
			}
		}
	}
}

func TestExplicitInheritedValueStaysPinnedUntilUnset(t *testing.T) {
	c := testConfig(t)
	c.Defaults = Overrides{Connections: Int(8), Retries: Int(9), Limit: Text("1MiB/s")}
	scope := Scope{Profile: "Default"}
	before, err := c.Resolve(scope)
	if err != nil {
		t.Fatal(err)
	}
	next, changed, err := c.Set(scope, Connections, Int(8))
	if err != nil || !changed {
		t.Fatal("creating an explicit inherited value was treated as a no-op", err)
	}
	next, _, err = next.Set(Scope{Global: true}, Connections, Int(16))
	if err != nil {
		t.Fatal(err)
	}
	r, err := next.Resolve(scope)
	if err != nil || r.Settings.Connections != 8 || r.Entries[0].Source != SourceProfile {
		t.Fatal("global edit replaced an explicit profile value", err)
	}

	for key, value := range (Overrides{Retries: Int(0), Limit: Text(Unlimited)}) {
		next, changed, err = next.Set(scope, key, value)
		if err != nil || !changed {
			t.Fatal("explicit zero/unlimited was not stored", err)
		}
	}
	r, err = next.Resolve(scope)
	if err != nil || r.Settings.Retries != 0 || r.Settings.LimitBPS != 0 {
		t.Fatal("explicit zero/unlimited inherited a nonzero global", err)
	}
	for _, key := range []Key{Connections, Retries, Limit} {
		next, changed, err = next.Unset(scope, key)
		if err != nil || !changed {
			t.Fatal("unset failed", err)
		}
	}
	r, err = next.Resolve(scope)
	if err != nil || r.Settings.Connections != 16 || r.Settings.Retries != 9 || r.Settings.LimitBPS != 1048576 {
		t.Fatal("unset did not restore current globals", err)
	}
	if before.Settings.Connections != 8 || before.Settings.Retries != 9 || before.Settings.LimitBPS != 1048576 {
		t.Fatal("edits changed an earlier resolved snapshot")
	}
}

func TestFailedEditsReturnNoCandidateAndPreserveInput(t *testing.T) {
	c := testConfig(t)
	c.Defaults = Overrides{Retries: Int(0)}
	before := c.Clone()
	for _, test := range []struct {
		scope  Scope
		key    Key
		value  Value
		remove bool
		err    error
	}{
		{Scope{}, Connections, Int(4), false, ErrScope},
		{Scope{Global: true, Profile: "Default"}, Connections, Int(4), false, ErrScope},
		{Scope{Profile: "Def"}, Connections, Int(4), false, ErrProfile},
		{Scope{Global: true}, "instance_id", Text(testID(99)), false, ErrKey},
		{Scope{Global: true}, "state_dir", Text(c.StateDir), false, ErrKey},
		{Scope{Profile: "Default"}, Connections, Text("4"), false, ErrValue},
		{Scope{Profile: "Default"}, Connections, Int(33), false, ErrValue},
		{Scope{Global: true}, OutputDir, Text("relative"), false, ErrValue},
		{Scope{Global: true}, Limit, Text("private-invalid"), false, ErrValue},
		{Scope{Global: true}, Limit, Value{}, false, ErrValue},
		{Scope{}, Connections, Value{}, true, ErrScope},
		{Scope{Global: true, Profile: "Default"}, Connections, Value{}, true, ErrScope},
		{Scope{Profile: testID(99)}, Connections, Value{}, true, ErrProfile},
		{Scope{Global: true}, "private-invalid", Value{}, true, ErrKey},
	} {
		var got Config
		var changed bool
		var err error
		if test.remove {
			got, changed, err = c.Unset(test.scope, test.key)
		} else {
			got, changed, err = c.Set(test.scope, test.key, test.value)
		}
		if err != test.err || changed || !reflect.DeepEqual(got, Config{}) || !reflect.DeepEqual(c, before) {
			t.Fatal("invalid edit returned partial data or modified its input", err)
		}
	}

	c.Profiles[0].Settings = Overrides{Connections: Int(0)}
	before = c.Clone()
	for _, remove := range []bool{false, true} {
		got, changed, err := c.edit(Scope{Global: true}, Retries, Int(3), remove)
		if err != ErrInvalid || changed || !reflect.DeepEqual(got, Config{}) || !reflect.DeepEqual(c, before) {
			t.Fatal("edit bypassed invalid existing configuration")
		}
	}
}

func TestChangedAndUnchangedEditsReturnOwnedConfigs(t *testing.T) {
	c := testConfig(t)
	c.Defaults = Overrides{Connections: Int(4)}
	c.Profiles[0].Settings = Overrides{Limit: Text(Unlimited)}
	c.Profiles = append(c.Profiles, Profile{ID: testID(3), Name: "Other", Settings: Overrides{Retries: Int(0)}})
	before := c.Clone()
	for _, test := range []struct {
		scope   Scope
		key     Key
		value   Value
		remove  bool
		changed bool
	}{
		{Scope{Global: true}, Connections, Int(4), false, false},
		{Scope{Global: true}, Timeout, Value{}, true, false},
		{Scope{Profile: "Default"}, Connections, Int(4), false, true},
		{Scope{Profile: "Default"}, Limit, Value{}, true, true},
	} {
		next, changed, err := c.edit(test.scope, test.key, test.value, test.remove)
		if err != nil || changed != test.changed {
			t.Fatal("unexpected edit result", err)
		}
		next.Defaults[Connections] = Int(32)
		next.Profiles[0].Settings[Limit] = Text("1B/s")
		next.Profiles[0].Name = "Changed"
		next.Profiles[1].Settings[Retries] = Int(10)
		if !reflect.DeepEqual(c, before) {
			t.Fatal("edit result shared config maps or profiles with its input")
		}
	}
}
