package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"

	"yoinkdl/internal/jsoncheck"
)

func decodeConfig(data []byte) (Config, error) {
	if len(data) > MaxConfigSize || !jsoncheck.ValidUnicode(data) {
		return Config{}, ErrInvalid
	}
	f, err := objectFields(data, "version", "instance_id", "state_dir", "defaults", "profiles")
	if err != nil || len(f) != 5 {
		return Config{}, ErrInvalid
	}
	var c Config
	if json.Unmarshal(f["version"], &c.Version) != nil ||
		json.Unmarshal(f["instance_id"], &c.InstanceID) != nil ||
		json.Unmarshal(f["state_dir"], &c.StateDir) != nil {
		return Config{}, ErrInvalid
	}
	c.Defaults, err = decodeOverrides(f["defaults"])
	if err != nil {
		return Config{}, err
	}
	c.Profiles, err = decodeProfiles(f["profiles"])
	if err != nil {
		return Config{}, err
	}
	if _, err := encodeConfig(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func decodeProfiles(data []byte) ([]Profile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('[') {
		return nil, ErrInvalid
	}
	var profiles []Profile
	for dec.More() {
		if len(profiles) == MaxProfiles {
			return nil, ErrInvalid
		}
		var data json.RawMessage
		if dec.Decode(&data) != nil {
			return nil, ErrInvalid
		}
		f, err := objectFields(data, "id", "name", "settings")
		if err != nil || len(f) != 3 {
			return nil, ErrInvalid
		}
		var p Profile
		if json.Unmarshal(f["id"], &p.ID) != nil || json.Unmarshal(f["name"], &p.Name) != nil {
			return nil, ErrInvalid
		}
		p.Settings, err = decodeOverrides(f["settings"])
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	if !jsonEnd(dec, ']') {
		return nil, ErrInvalid
	}
	return profiles, nil
}

func decodeOverrides(data []byte) (Overrides, error) {
	groups, err := objectFields(data, "download", "storage")
	if err != nil {
		return nil, err
	}
	var values Overrides
	for group, data := range groups {
		prefix := group + "."
		var names []string
		for _, key := range keys {
			if name, found := strings.CutPrefix(string(key), prefix); found {
				names = append(names, name)
			}
		}
		fields, err := objectFields(data, names...)
		if err != nil {
			return nil, err
		}
		for name, data := range fields {
			key := Key(prefix + name)
			var value Value
			if key == Connections || key == Retries {
				var n int
				if json.Unmarshal(data, &n) != nil {
					return nil, ErrInvalid
				}
				value = Int(n)
			} else {
				var text string
				if json.Unmarshal(data, &text) != nil {
					return nil, ErrInvalid
				}
				value = Text(text)
			}
			if values == nil {
				values = make(Overrides)
			}
			values[key] = value
		}
	}
	return values, nil
}

func objectFields(data []byte, names ...string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, ErrInvalid
	}
	fields := make(map[string]json.RawMessage, len(names))
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || !slices.Contains(names, name) || fields[name] != nil {
			return nil, ErrInvalid
		}
		var data json.RawMessage
		if dec.Decode(&data) != nil || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			return nil, ErrInvalid
		}
		fields[name] = data
	}
	if !jsonEnd(dec, '}') {
		return nil, ErrInvalid
	}
	return fields, nil
}

func jsonEnd(dec *json.Decoder, end json.Delim) bool {
	if token, err := dec.Token(); err != nil || token != end {
		return false
	}
	_, err := dec.Token()
	return errors.Is(err, io.EOF)
}
