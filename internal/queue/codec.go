package queue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"yoinkdl/internal/jsoncheck"
)

var errJSON = errors.New("invalid queue JSON fields or values")

func encodeSnapshot(s Snapshot) ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(s)
	if err != nil || len(data) >= MaxSnapshotSize {
		return nil, errors.New("queue snapshot exceeds encoding limits")
	}

	return append(data, '\n'), nil
}

func decodeSnapshot(data []byte) (Snapshot, error) {
	if len(data) > MaxSnapshotSize || !jsoncheck.ValidUnicode(data) {
		return Snapshot{}, errJSON
	}

	fields, err := objectFields(data, "version", "parallel", "next_order", "jobs")
	if err != nil || !nonNull(fields, "version", "parallel", "next_order", "jobs") {
		return Snapshot{}, errJSON
	}

	dec := json.NewDecoder(bytes.NewReader(fields["jobs"]))
	if token, err := dec.Token(); err != nil || token != json.Delim('[') {
		return Snapshot{}, errJSON
	}

	count := 0
	for dec.More() {
		if count == MaxJobs {
			return Snapshot{}, errors.New("queue contains too many jobs")
		}

		var item json.RawMessage
		if err := dec.Decode(&item); err != nil {
			return Snapshot{}, errJSON
		}

		job, err := objectFields(item, "id", "order", "request", "input_hash", "state", "published", "error")
		if err != nil || !nonNull(job, "id", "order", "request", "input_hash", "state") {
			return Snapshot{}, errJSON
		}

		req, err := objectFields(job["request"], "url", "output", "connections", "retries", "timeout_ns", "limit_bps", "resume")
		if err != nil || !nonNull(req, "output", "connections", "retries", "timeout_ns", "limit_bps", "resume") {
			return Snapshot{}, errJSON
		}

		count++
	}

	if token, err := dec.Token(); err != nil || token != json.Delim(']') {
		return Snapshot{}, errJSON
	}

	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Snapshot{}, errJSON
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, errJSON
	}

	if err := s.Validate(); err != nil {
		return Snapshot{}, err
	}

	return s, nil
}

func objectFields(data []byte, names ...string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, errJSON
	}

	fields := make(map[string]json.RawMessage, len(names))
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || !slices.Contains(names, name) || fields[name] != nil {
			return nil, errJSON
		}

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, errJSON
		}

		fields[name] = value
	}

	if token, err := dec.Token(); err != nil || token != json.Delim('}') || len(fields) != len(names) {
		return nil, errJSON
	}

	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errJSON
	}

	return fields, nil
}

func nonNull(fields map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		if fields[name] == nil || bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) {
			return false
		}
	}

	return true
}
