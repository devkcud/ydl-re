package app

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSnapshotCodecPreservesDefaultsAndLargeIntegers(t *testing.T) {
	initial := testSnapshot(t)
	s := testSubmitted(t)
	s.Profiles[0].Revision = 9007199254740993
	s.Profiles[0].TimeoutNS = math.MaxInt64
	s.Submissions[0].ProfileRevision = s.Profiles[0].Revision
	s.Submissions[0].Settings.LimitBPS = math.MaxInt64
	for _, want := range []Snapshot{initial, s} {
		data := encoded(t, want)
		got, err := decodeSnapshot(data)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("round trip changed metadata", err)
		}
		if data[len(data)-1] != '\n' {
			t.Fatal("snapshot lacks final newline")
		}
	}
	data := string(encoded(t, s))
	for _, field := range []string{`"revision":"9007199254740993"`, `"profile_revision":"9007199254740993"`,
		`"timeout_ns":"9223372036854775807"`, `"limit_bps":"9223372036854775807"`} {
		if !strings.Contains(data, field) {
			t.Fatal("64-bit value was not preserved as a decimal string")
		}
	}
}

func TestSnapshotCodecRequiresExactFields(t *testing.T) {
	data := encoded(t, testSubmitted(t))
	for _, level := range []string{"snapshot", "profile", "submission", "settings"} {
		root, fields := jsonFields(t, data, level)
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			t.Run(level+"/"+key, func(t *testing.T) {
				value := fields[key]
				delete(fields, key)
				refuseJSON(t, marshaled(t, root))
				if level != "profile" || key != "output_dir" {
					fields[key] = nil
					refuseJSON(t, marshaled(t, root))
				}
				fields[key] = value
			})
		}
		fields["url"] = "https://example.invalid/?token=private"
		refuseJSON(t, marshaled(t, root))
	}
}

func TestSnapshotCodecRejectsMalformedJSON(t *testing.T) {
	base := string(encoded(t, testSubmitted(t)))
	for _, data := range []string{
		"", "null", "[]", "{}", base + "{}", base[:len(base)-2], base + "\xff",
		strings.Replace(base, `"version":1`, `"version":1.0`, 1),
		strings.Replace(base, `"version":1`, `"version":2`, 1),
		strings.Replace(base, `"version":1`, `"Version":1`, 1),
		strings.Replace(base, `"version":1`, `"version":1,"\u0076ersion":1`, 1),
		strings.Replace(base, `"revision":"1"`, `"revision":"1","revision":"1"`, 1),
		strings.Replace(base, `"profile_revision":"1"`, `"profile_revision":"1","profile_revision":"1"`, 1),
		strings.Replace(base, `"resume":false`, `"resume":false,"resume":true`, 1),
		strings.Replace(base, `"resume":false`, `"resume":"false"`, 1),
		strings.Replace(base, `"connections":4`, `"connections":"4"`, 1),
		strings.Replace(base, `"connections":4`, `"connections":4.0`, 1),
		strings.Replace(base, `"profiles":[`, `"profiles":[null,`, 1),
		strings.Replace(base, `"submissions":[`, `"submissions":[null,`, 1),
	} {
		refuseJSON(t, []byte(data))
	}
	for _, level := range []string{"snapshot", "submission"} {
		root, fields := jsonFields(t, []byte(base), level)
		for _, key := range []string{"profiles", "submissions", "settings"} {
			if value, found := fields[key]; found {
				fields[key] = "invalid"
				refuseJSON(t, marshaled(t, root))
				fields[key] = value
			}
		}
	}
}

func TestSnapshotCodecRejectsNoncanonicalIntegers(t *testing.T) {
	data := encoded(t, testSubmitted(t))
	for _, level := range []string{"profile", "submission", "settings"} {
		root, fields := jsonFields(t, data, level)
		for _, key := range []string{"revision", "profile_revision", "timeout_ns", "limit_bps"} {
			original, found := fields[key]
			if !found {
				continue
			}
			for _, value := range []any{1, true, "", "01", "+1", "-0", "-1", "1.0", "1e3", " 1", "١", "9223372036854775808"} {
				fields[key] = value
				refuseJSON(t, marshaled(t, root))
			}
			fields[key] = original
		}
	}
}

func TestSnapshotCodecRejectsLossyUnicode(t *testing.T) {
	base := string(encoded(t, testSnapshot(t)))
	for _, value := range []string{`"\ud800"`, `"\udc00"`, `"\ud800\u0020"`, `"\udc00\ud800"`, `"\uzzzz"`} {
		refuseJSON(t, []byte(strings.Replace(base, `"Default"`, value, 1)))
	}
	for _, value := range []string{`"\ud83d\ude00"`, `"\\ud800"`, `"\ufffd"`} {
		if _, err := decodeSnapshot([]byte(strings.Replace(base, `"Default"`, value, 1))); err != nil {
			t.Fatal("rejected valid Unicode", err)
		}
	}
}

func TestSnapshotCodecBounds(t *testing.T) {
	data := encoded(t, testSnapshot(t))
	padded := append(data, bytes.Repeat([]byte{' '}, MaxSnapshotSize-len(data))...)
	if _, err := decodeSnapshot(padded); err != nil {
		t.Fatal("rejected snapshot at the input byte limit", err)
	}
	refuseJSON(t, append(padded, ' '))
	for _, profiles := range []bool{true, false} {
		s := testSubmitted(t)
		if profiles {
			for i := 1; i < MaxProfiles; i++ {
				p := s.Profiles[0]
				p.ID = testID(i + 2)
				s.Profiles = append(s.Profiles, p)
			}
			if _, err := decodeSnapshot(encoded(t, s)); err != nil {
				t.Fatal("rejected the maximum profile count", err)
			}
			p := s.Profiles[0]
			p.ID = testID(1000)
			s.Profiles = append(s.Profiles, p)
		} else {
			for i := 1; i < MaxSubmissions; i++ {
				sub := s.Submissions[0]
				sub.ID = testID(i + 3)
				s.Submissions = append(s.Submissions, sub)
			}
			if _, err := decodeSnapshot(encoded(t, s)); err != nil {
				t.Fatal("rejected the maximum submission count", err)
			}
			sub := s.Submissions[0]
			sub.ID = testID(1000)
			s.Submissions = append(s.Submissions, sub)
		}
		refuseJSON(t, marshaled(t, s))
	}

	s := testSubmitted(t)
	dir := filepath.Join(filepath.Dir(s.StateDir), strings.Repeat("<", 3000))
	s.Profiles[0].OutputDir = &dir
	s.Submissions[0].Settings.Output = filepath.Join(dir, "file")
	for i := 1; i < MaxSubmissions; i++ {
		sub := s.Submissions[0]
		sub.ID = testID(i + 3)
		s.Submissions = append(s.Submissions, sub)
	}
	if err := s.validate(); err != nil {
		t.Fatal("encoding-limit fixture must have valid fields", err)
	}
	if data, err := encodeSnapshot(s); err != ErrInvalid || data != nil || s.Validate() != ErrInvalid {
		t.Fatal("encoder accepted oversized escaped metadata or exposed partial output")
	}
	compact := bytes.ReplaceAll(marshaled(t, s), []byte(`\u003c`), []byte("<"))
	if len(compact) > MaxSnapshotSize {
		t.Fatal("compact fixture must fit the input byte limit")
	}
	refuseJSON(t, compact)
}

func FuzzSnapshotCodec(f *testing.F) {
	dir := filepath.Join(f.TempDir(), "state")
	s, err := NewSnapshot(testID(1), testID(2), dir)
	if err != nil {
		f.Fatal(err)
	}
	data, err := encodeSnapshot(s)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	s.Profiles[0].OutputDir = &dir
	s.Submissions = []Submission{{ID: testID(3), ProfileID: testID(2), ProfileRevision: 1,
		Settings: Settings{Output: filepath.Join(dir, "file"), Defaults: s.Profiles[0].Defaults}}}
	data, err = encodeSnapshot(s)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := decodeSnapshot(data)
		if err != nil {
			if err != ErrInvalid || !reflect.DeepEqual(s, Snapshot{}) {
				t.Fatal("invalid input exposed partial state or an unsafe error")
			}
			return
		}
		got, err := decodeSnapshot(encoded(t, s))
		if err != nil || !reflect.DeepEqual(got, s) {
			t.Fatal("valid snapshot failed canonical round trip", err)
		}
	})
}

func encoded(t *testing.T, s Snapshot) []byte {
	t.Helper()
	data, err := encodeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func marshaled(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func refuseJSON(t *testing.T, data []byte) {
	t.Helper()
	got, err := decodeSnapshot(data)
	if err != ErrInvalid || !reflect.DeepEqual(got, Snapshot{}) {
		t.Fatal("invalid input returned partial metadata or an unsafe error", err)
	}
}

func jsonFields(t *testing.T, data []byte, level string) (map[string]any, map[string]any) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	fields := root
	switch level {
	case "profile":
		fields = root["profiles"].([]any)[0].(map[string]any)
	case "submission", "settings":
		fields = root["submissions"].([]any)[0].(map[string]any)
		if level == "settings" {
			fields = fields["settings"].(map[string]any)
		}
	}
	return root, fields
}
