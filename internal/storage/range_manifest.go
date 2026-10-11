package storage

import (
	"encoding/json"
	"errors"

	"yoinkdl/internal/ranges"
)

type Chunk struct {
	ranges.Span
	Checksum string `json:"sha256"`
}

type rangeManifest struct {
	Version     int     `json:"schema_version"`
	Destination string  `json:"destination"`
	SourceID    string  `json:"source_id"`
	EffectiveID string  `json:"effective_id"`
	Length      int64   `json:"length"`
	ETag        string  `json:"etag"`
	Encoding    string  `json:"encoding"`
	ChunkSize   int64   `json:"chunk_size"`
	Ranges      []Chunk `json:"completed_ranges"`
	Checksum    string  `json:"sha256"`
	State       string  `json:"state"`
}

func (m Manifest) MarshalJSON() ([]byte, error) {
	if m.Version != RangeManifestVersion {
		type prefixManifest Manifest
		return json.Marshal(prefixManifest(m))
	}

	return json.Marshal(rangeManifest{
		Version: m.Version, Destination: m.Destination, SourceID: m.SourceID,
		EffectiveID: m.EffectiveID, Length: m.Length, ETag: m.ETag, Encoding: m.Encoding,
		ChunkSize: m.ChunkSize, Ranges: m.Ranges, Checksum: m.Checksum, State: m.State,
	})
}

func decodeManifest(data []byte) (Manifest, error) {
	fields, err := objectFields(data)
	if err != nil {
		return Manifest{}, err
	}

	var version int
	if err := json.Unmarshal(fields["schema_version"], &version); err != nil {
		return Manifest{}, errors.New("invalid manifest version")
	}

	names := []string{"schema_version", "destination", "source_id", "effective_id", "length", "etag", "encoding", "sha256", "state"}

	switch version {
	case ManifestVersion:
		if !hasFields(fields, append(names, "completed")...) {
			return Manifest{}, errors.New("invalid version-1 manifest fields")
		}

		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return Manifest{}, errors.New("invalid prefix manifest JSON")
		}

		return m, nil

	case RangeManifestVersion:
		if !hasFields(fields, append(names, "chunk_size", "completed_ranges")...) {
			return Manifest{}, errors.New("invalid version-2 manifest fields")
		}

		var items []json.RawMessage
		if err := json.Unmarshal(fields["completed_ranges"], &items); err != nil || len(items) > ranges.MaxChunks {
			return Manifest{}, errors.New("invalid completed ranges")
		}

		for _, item := range items {
			fields, err := objectFields(item)
			if err != nil || !hasFields(fields, "start", "end", "sha256") {
				return Manifest{}, errors.New("invalid completed range fields")
			}
		}

		var saved rangeManifest
		if err := json.Unmarshal(data, &saved); err != nil {
			return Manifest{}, errors.New("invalid range manifest JSON")
		}

		m := Manifest{
			Version: saved.Version, Destination: saved.Destination, SourceID: saved.SourceID,
			EffectiveID: saved.EffectiveID, Length: saved.Length, ETag: saved.ETag, Encoding: saved.Encoding,
			ChunkSize: saved.ChunkSize, Ranges: saved.Ranges, Checksum: saved.Checksum, State: saved.State,
		}
		m.Completed, err = ranges.Covered(m.Length, m.ChunkSize, m.Spans())
		return m, err

	default:
		return Manifest{}, errors.New("unsupported manifest version")
	}
}

func (m Manifest) Spans() []ranges.Span {
	spans := make([]ranges.Span, len(m.Ranges))
	for i, chunk := range m.Ranges {
		spans[i] = chunk.Span
	}

	return spans
}

func (m Manifest) validateRanges() error {
	if m.Length < 0 || m.Encoding != "identity" || !StrongETag(m.ETag) || m.Ranges == nil {
		return errors.New("range state requires a known length, identity encoding, strong ETag, and chunk array")
	}

	covered, err := ranges.Covered(m.Length, m.ChunkSize, m.Spans())
	if err != nil {
		return err
	}

	if covered != m.Completed {
		return errors.New("saved byte count does not match completed ranges")
	}

	for _, chunk := range m.Ranges {
		if !validHash(chunk.Checksum) {
			return errors.New("invalid chunk checksum")
		}
	}

	switch m.State {
	case StatePartial:
		if m.Checksum != "" {
			return errors.New("partial range state cannot assert a full-file checksum")
		}
	case StateReady:
		if covered != m.Length || !validHash(m.Checksum) {
			return errors.New("ready range state requires exact coverage and a full-file checksum")
		}
	default:
		return errors.New("invalid checkpoint state")
	}

	return nil
}
