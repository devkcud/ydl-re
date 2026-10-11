package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestManifestSchemasStayDistinct(t *testing.T) {
	for _, version := range []int{ManifestVersion, RangeManifestVersion} {
		var path string
		var f *File

		if version == ManifestVersion {
			path, f = startTransfer(t, 10)
			writeBytes(t, f, "abc")
		} else {
			path, f = startRanges(t, 10, 3)
			putChunk(t, f, 6, "ghi")
		}

		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		data := readBytes(t, path+ManifestSuffix)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}

		if version == ManifestVersion {
			if len(fields) != 10 || string(fields["completed"]) != "3" || fields["completed_ranges"] != nil || fields["chunk_size"] != nil {
				t.Fatalf("version-1 format changed: %s", data)
			}
		} else if len(fields) != 11 || fields["completed"] != nil || fields["completed_ranges"] == nil || string(fields["chunk_size"]) != "3" {
			t.Fatalf("invalid version-2 format: %s", data)
		}

		m := inspect(t, path)
		if m.Version != version || m.Completed != 3 {
			t.Fatalf("version or saved-byte interpretation changed: %+v", m)
		}
	}
}

func TestRangeManifestValidation(t *testing.T) {
	cases := map[string]func(*Manifest){
		"unknown version":       func(m *Manifest) { m.Version = 3 },
		"unknown length":        func(m *Manifest) { m.Length = -1 },
		"missing validator":     func(m *Manifest) { m.ETag = "" },
		"encoded":               func(m *Manifest) { m.Encoding = "encoded" },
		"zero chunk":            func(m *Manifest) { m.ChunkSize = 0 },
		"negative chunk":        func(m *Manifest) { m.ChunkSize = -1 },
		"too many chunks":       func(m *Manifest) { m.Length = 3 * (ranges.MaxChunks + 1) },
		"no array":              func(m *Manifest) { m.Ranges = nil },
		"empty interval":        func(m *Manifest) { m.Ranges[0].End = 0 },
		"out of bounds":         func(m *Manifest) { m.Ranges[0].End = 11 },
		"partial chunk":         func(m *Manifest) { m.Ranges[0].End = 2 },
		"merged chunks":         func(m *Manifest) { m.Ranges[0].End = 6 },
		"checksum":              func(m *Manifest) { m.Ranges[0].Checksum = "invalid" },
		"unsorted":              func(m *Manifest) { m.Ranges[0], m.Ranges[1] = m.Ranges[1], m.Ranges[0] },
		"duplicate":             func(m *Manifest) { m.Ranges[1] = m.Ranges[0] },
		"partial full checksum": func(m *Manifest) { m.Checksum = hashText("anything") },
		"ready gap":             func(m *Manifest) { m.State = StateReady; m.Checksum = hashText("anything") },
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			path, f := startRanges(t, 10, 3)
			putChunk(t, f, 0, "abc")
			putChunk(t, f, 6, "ghi")
			crash(t, f)
			m := inspect(t, path)
			change(&m)
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}

			write(t, path+ManifestSuffix, string(data))
			before := readBytes(t, path+PartSuffix)
			if _, err := Inspect(path); err == nil {
				t.Fatal("accepted invalid range metadata")
			}

			if f, err := Open(context.Background(), path, true, true); err == nil {
				f.Close()
				t.Fatal("opened invalid range metadata")
			}

			if !bytes.Equal(before, readBytes(t, path+PartSuffix)) || !bytes.Equal(data, readBytes(t, path+ManifestSuffix)) {
				t.Fatal("changed invalid state while refusing it")
			}
		})
	}
}

func TestRangeManifestStrictJSON(t *testing.T) {
	path, f := startRanges(t, 10, 3)
	putChunk(t, f, 0, "abc")
	crash(t, f)
	data := readBytes(t, path+ManifestSuffix)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}

	for key := range fields {
		for _, mode := range []string{"null", "missing", "duplicate", "case"} {
			t.Run(key+"/"+mode, func(t *testing.T) {
				var copy map[string]json.RawMessage
				if err := json.Unmarshal(data, &copy); err != nil {
					t.Fatal(err)
				}

				switch mode {
				case "null":
					copy[key] = json.RawMessage("null")
				case "missing":
					delete(copy, key)
				case "case":
					copy[strings.ToUpper(key)] = copy[key]
					delete(copy, key)
				}

				raw, err := json.Marshal(copy)
				if err != nil {
					t.Fatal(err)
				}

				if mode == "duplicate" {
					raw = append([]byte(`{"`+key+`":`+string(fields[key])+`,`), raw[1:]...)
				}

				write(t, path+ManifestSuffix, string(raw))
				if _, err := Inspect(path); err == nil {
					t.Fatalf("accepted malformed field %s: %s", key, raw)
				}
			})
		}
	}

	for _, item := range []string{
		`null`, `[]`, `[0,3]`, `3`, `{}`, `{"start":null,"end":3,"sha256":"HASH"}`,
		`{"start":0,"start":0,"end":3,"sha256":"HASH"}`,
		`{"start":0,"end":3,"sha256":null}`,
		`{"start":0,"end":3,"SHA256":"HASH"}`,
		`{"start":0,"end":3,"sha256":"HASH","extra":true}`,
		`{"start":0.5,"end":3,"sha256":"HASH"}`,
		`{"start":0,"end":9223372036854775808,"sha256":"HASH"}`,
	} {
		fields["completed_ranges"] = json.RawMessage("[" + strings.ReplaceAll(item, "HASH", hashText("abc")) + "]")
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}

		write(t, path+ManifestSuffix, string(raw))
		if _, err := Inspect(path); err == nil {
			t.Fatalf("accepted malformed range: %s", item)
		}
	}
}

func TestRangeManifestCapacity(t *testing.T) {
	path, f := startRanges(t, math.MaxInt64, 0)
	m, _ := f.Metadata()
	m.ETag = `"` + strings.Repeat("x", 4094) + `"`
	plan, err := ranges.Missing(m.Length, m.ChunkSize, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, span := range plan {
		m.Ranges = append(m.Ranges, Chunk{Span: span, Checksum: hashText("chunk")})
	}

	m.Completed = m.Length
	if err := m.validate(path); err != nil {
		t.Fatal(err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil || len(data)+1 > maxManifestSize {
		t.Fatalf("bounded plan exceeds manifest size: %d bytes, error = %v", len(data), err)
	}

	write(t, path+ManifestSuffix, string(data))
	if got := inspect(t, path); got.Completed != math.MaxInt64 || len(got.Ranges) != ranges.MaxChunks {
		t.Fatalf("lost large-file coverage: %+v", got)
	}
}

func TestReadyRangeRecoveryChecksFullDigest(t *testing.T) {
	path, f := startRanges(t, 3, 3)
	putChunk(t, f, 0, "abc")
	if err := f.prepare(); err != nil {
		t.Fatal(err)
	}
	crash(t, f)
	m := inspect(t, path)
	m.Checksum = hashText("bad")
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path+ManifestSuffix, string(data))

	if f, err := Open(context.Background(), path, false, true); err == nil {
		f.Close()
		t.Fatal("range digests bypassed a corrupt full-file digest")
	}

	assertAbsent(t, path)
	assertData(t, path+PartSuffix, "abc")
	if !bytes.Equal(data, readBytes(t, path+ManifestSuffix)) {
		t.Fatal("failed ready recovery changed metadata")
	}
}

func TestBeginRangesRejectsIneligibleIdentity(t *testing.T) {
	for _, mode := range []string{"unknown", "encoded", "unversioned", "weak", "prefix version"} {
		t.Run(mode, func(t *testing.T) {
			path := t.TempDir() + "/file"
			f, err := Create(path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			m := Manifest{SourceID: hashText("source"), EffectiveID: hashText("effective"), Length: 10, ETag: `"v1"`, Encoding: "identity"}

			switch mode {
			case "unknown":
				m.Length = -1
			case "encoded":
				m.Encoding = "encoded"
			case "unversioned":
				m.ETag = ""
			case "weak":
				m.ETag = `W/"v1"`
			case "prefix version":
				m.Version = ManifestVersion
			}

			if err := f.BeginRanges(m); err == nil {
				t.Fatal("created range state without eligible identity")
			}

			if _, err := os.Stat(path + ManifestSuffix); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("ineligible identity left a manifest: %v", err)
			}
		})
	}
}
