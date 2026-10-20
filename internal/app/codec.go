package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
)

func encodeSnapshot(s Snapshot) ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(s)
	if err != nil || len(data) >= MaxSnapshotSize {
		return nil, ErrInvalid
	}
	return append(data, '\n'), nil
}

func decodeSnapshot(data []byte) (Snapshot, error) {
	if len(data) > MaxSnapshotSize || !validJSONText(data) {
		return Snapshot{}, ErrInvalid
	}
	fields, err := objectFields(data, "", "version", "instance_id", "state_dir", "profiles", "submissions")
	if err != nil {
		return Snapshot{}, err
	}
	if err := checkArray(fields["profiles"], MaxProfiles, checkProfile); err != nil {
		return Snapshot{}, err
	}
	if err := checkArray(fields["submissions"], MaxSubmissions, checkSubmission); err != nil {
		return Snapshot{}, err
	}

	var s Snapshot
	if json.Unmarshal(data, &s) != nil {
		return Snapshot{}, ErrInvalid
	}
	if err := s.Validate(); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func checkProfile(data []byte) error {
	f, err := objectFields(data, "output_dir", "id", "revision", "name", "output_dir",
		"connections", "retries", "timeout_ns", "limit_bps")
	if err != nil || !decimal(f["revision"]) || !decimal(f["timeout_ns"]) || !decimal(f["limit_bps"]) {
		return ErrInvalid
	}
	return nil
}

func checkSubmission(data []byte) error {
	f, err := objectFields(data, "", "id", "profile_id", "profile_revision", "settings")
	if err != nil || !decimal(f["profile_revision"]) {
		return ErrInvalid
	}
	f, err = objectFields(f["settings"], "", "output", "connections", "retries", "timeout_ns", "limit_bps", "resume")
	if err != nil || !decimal(f["timeout_ns"]) || !decimal(f["limit_bps"]) {
		return ErrInvalid
	}
	return nil
}

func decimal(data []byte) bool {
	var value string
	if json.Unmarshal(data, &value) != nil {
		return false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return err == nil && n >= 0 && strconv.FormatInt(n, 10) == value
}

func objectFields(data []byte, nullable string, names ...string) (map[string]json.RawMessage, error) {
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
		var value json.RawMessage
		if dec.Decode(&value) != nil || name != nullable && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrInvalid
		}
		fields[name] = value
	}
	if token, err := dec.Token(); err != nil || token != json.Delim('}') || len(fields) != len(names) {
		return nil, ErrInvalid
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, ErrInvalid
	}
	return fields, nil
}

func checkArray(data []byte, limit int, check func([]byte) error) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('[') {
		return ErrInvalid
	}
	count := 0
	for dec.More() {
		if count == limit {
			return ErrInvalid
		}
		var item json.RawMessage
		if dec.Decode(&item) != nil || check(item) != nil {
			return ErrInvalid
		}
		count++
	}
	if token, err := dec.Token(); err != nil || token != json.Delim(']') {
		return ErrInvalid
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ErrInvalid
	}
	return nil
}
