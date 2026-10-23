package config

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestConfigCodecRequiresExactFieldsAndTypes(t *testing.T) {
	data := encodedConfig(t, codecConfig(t))
	for _, level := range []string{"root", "profile", "defaults", "settings", "global-download", "global-storage", "profile-download", "profile-storage"} {
		t.Run(level, func(t *testing.T) {
			root, fields := configFields(t, data, level)
			var names []string
			for key := range fields {
				names = append(names, key)
			}
			slices.Sort(names)
			for _, key := range names {
				original := fields[key]
				delete(fields, key)
				if level == "root" || level == "profile" {
					refuseConfigJSON(t, configJSONBytes(t, root))
				} else if _, err := decodeConfig(configJSONBytes(t, root)); err != nil {
					t.Fatal("optional override became required", key, err)
				}
				fields[key] = nil
				refuseConfigJSON(t, configJSONBytes(t, root))
				fields[key] = true
				refuseConfigJSON(t, configJSONBytes(t, root))
				fields[key] = []any{}
				refuseConfigJSON(t, configJSONBytes(t, root))
				fields[key] = original
			}
			fields["source_url"] = "https://example.invalid/?private=token"
			refuseConfigJSON(t, configJSONBytes(t, root))
		})
	}
}

func TestConfigCodecRejectsDuplicateAndMalformedJSON(t *testing.T) {
	data := encodedConfig(t, codecConfig(t))
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		t.Fatal(err)
	}
	base := compact.String()
	for _, test := range []struct{ old, next string }{
		{`"version":1`, `"version":1,"\u0076ersion":1`},
		{`"version":1`, `"Version":1`},
		{`"defaults":`, `"defaults":{},"defaults":`},
		{`"download":`, `"download":{},"download":`},
		{`"storage":`, `"storage":{},"storage":`},
		{`"settings":`, `"settings":{},"settings":`},
		{`"connections":4`, `"connections":4,"\u0063onnections":4`},
		{`"staging_dir":"destination"`, `"staging_dir":"destination","staging_dir":"destination"`},
		{`"id":"` + testID(2) + `"`, `"id":"` + testID(2) + `","id":"` + testID(2) + `"`},
		{`"id":"` + testID(3) + `"`, `"id":"` + testID(2) + `"`},
		{`"Pato 🐈"`, `"Default"`},
		{`"profiles":[`, `"profiles":[null,`},
		{`"retries":0`, `"retries":"0"`},
		{`"limit":"5MiB/s"`, `"limit":0`},
	} {
		changed := strings.Replace(base, test.old, test.next, 1)
		if changed == base {
			t.Fatal("mutation did not change the fixture", test.old)
		}
		refuseConfigJSON(t, []byte(changed))
	}
	for _, bad := range []string{"", "null", "[]", "{}", base + "{}", base + "false", base[:len(base)-1], base + "\xff", "\ufeff" + base, "/* comment */" + base} {
		refuseConfigJSON(t, []byte(bad))
	}
	for _, test := range []struct {
		old    string
		values []string
	}{
		{`"version":1`, []string{`1.0`, `1e0`, `"1"`, `true`, `2`, `-1`}},
		{`"connections":4`, []string{`4.0`, `4e0`, `"4"`, `false`, `0`, `33`, `+4`, `04`, `9999999999999999999999`}},
		{`"retries":0`, []string{`0.0`, `0e0`, `"0"`, `false`, `-1`, `11`}},
	} {
		key, _, _ := strings.Cut(test.old, ":")
		for _, value := range test.values {
			refuseConfigJSON(t, []byte(strings.Replace(base, test.old, key+":"+value, 1)))
		}
	}
	if _, err := decodeConfig([]byte(strings.Replace(base, `"retries":0`, `"retries":-0`, 1))); err != nil {
		t.Fatal("JSON integer zero was refused", err)
	}
}

func TestConfigCodecRejectsLossyUnicode(t *testing.T) {
	base := string(encodedConfig(t, testConfig(t)))
	for _, value := range []string{`"\ud800"`, `"\udc00"`, `"\ud800\u0020"`, `"\udc00\ud800"`, `"\uzzzz"`, "\"\xff\""} {
		refuseConfigJSON(t, []byte(strings.Replace(base, `"Default"`, value, 1)))
	}
	for _, value := range []string{`"\ud83d\ude00"`, `"\\ud800"`, `"\ufffd"`, `"猫"`} {
		if _, err := decodeConfig([]byte(strings.Replace(base, `"Default"`, value, 1))); err != nil {
			t.Fatal("valid Unicode was refused", err)
		}
	}
}

func configFields(t *testing.T, data []byte, level string) (map[string]any, map[string]any) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	profile := root["profiles"].([]any)[0].(map[string]any)
	defaults := root["defaults"].(map[string]any)
	settings := profile["settings"].(map[string]any)
	switch level {
	case "root":
		return root, root
	case "profile":
		return root, profile
	case "defaults":
		return root, defaults
	case "settings":
		return root, settings
	case "global-download":
		return root, defaults["download"].(map[string]any)
	case "global-storage":
		return root, defaults["storage"].(map[string]any)
	case "profile-download":
		return root, settings["download"].(map[string]any)
	case "profile-storage":
		return root, settings["storage"].(map[string]any)
	default:
		t.Fatal("unknown fixture level")
		return nil, nil
	}
}
