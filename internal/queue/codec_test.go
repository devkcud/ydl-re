package queue

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestSnapshotStrictJSON(t *testing.T) {
	s := testSnapshot(t, t.TempDir(), 1)
	data, err := encodeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)

	for _, test := range []struct {
		name string
		data string
	}{
		{"null", "null"},
		{"array", "[]"},
		{"empty", "{}"},
		{"trailing", base + "{}"},
		{"version null", strings.Replace(base, `"version":1`, `"version":null`, 1)},
		{"version float", strings.Replace(base, `"version":1`, `"version":1.0`, 1)},
		{"version case", strings.Replace(base, `"version"`, `"Version"`, 1)},
		{"version duplicate", strings.Replace(base, `"version":1`, `"version":1,"version":1`, 1)},
		{"escaped duplicate", strings.Replace(base, `"version":1`, `"version":1,"\u0076ersion":1`, 1)},
		{"unknown", strings.Replace(base, `"version":1`, `"queue-private":1,"version":1`, 1)},
		{"settings missing", strings.Replace(base, `"parallel":1,`, "", 1)},
		{"settings null", strings.Replace(base, `"parallel":1`, `"parallel":null`, 1)},
		{"jobs null", `{"version":1,"parallel":1,"next_order":1,"jobs":null}`},
		{"job null", `{"version":1,"parallel":1,"next_order":2,"jobs":[null]}`},
		{"jobs object", `{"version":1,"parallel":1,"next_order":1,"jobs":{}}`},
		{"order overflow", strings.Replace(base, `"next_order":2`, `"next_order":9223372036854775808`, 1)},
		{"request null", strings.Replace(base, `"request":{`, `"request":null,"duplicate":{`, 1)},
		{"request unknown", strings.Replace(base, `"resume":false`, `"resume":false,"queue-private":true`, 1)},
		{"bool null", strings.Replace(base, `"resume":false`, `"resume":null`, 1)},
		{"bool string", strings.Replace(base, `"resume":false`, `"resume":"false"`, 1)},
		{"bool duplicate", strings.Replace(base, `"resume":false`, `"resume":false,"resume":true`, 1)},
		{"published missing", strings.Replace(base, `"published":false,`, "", 1)},
		{"published string", strings.Replace(base, `"published":false`, `"published":"queue-private"`, 1)},
		{"nullable duplicate", strings.Replace(base, `"error":null`, `"error":null,"error":null`, 1)},
		{"error missing", strings.Replace(base, `,"error":null`, "", 1)},
		{"error number", strings.Replace(base, `"error":null`, `"error":1`, 1)},
		{"UTF8", base + "\xff"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeSnapshot([]byte(test.data))
			assertPrivateError(t, err)
			if !reflect.DeepEqual(got, Snapshot{}) {
				t.Fatal("failed decoding returned private partial state")
			}
		})
	}
}

func TestSnapshotAllRequiredFields(t *testing.T) {
	s := testSnapshot(t, t.TempDir(), 1)
	data, err := encodeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}

	for _, level := range []string{"snapshot", "job", "request"} {
		t.Run(level, func(t *testing.T) {
			var original map[string]any
			if err := json.Unmarshal(data, &original); err != nil {
				t.Fatal(err)
			}

			fields := original
			if level != "snapshot" {
				fields = original["jobs"].([]any)[0].(map[string]any)
			}
			if level == "request" {
				fields = fields["request"].(map[string]any)
			}

			keys := make([]string, 0, len(fields))
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				value := fields[key]
				delete(fields, key)
				missing, err := json.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}

				if _, err := decodeSnapshot(missing); err == nil {
					t.Fatalf("accepted missing %s field", key)
				}

				fields[key] = value
			}
		})
	}
}

func TestSnapshotRejectsLossyUnicode(t *testing.T) {
	s := testSnapshot(t, t.TempDir(), 1)
	s.Jobs[0].State = Failed
	s.Jobs[0].Error = ptr("safe")
	data, err := encodeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{`"\ud800"`, `"\udc00"`, `"\ud800\u0020"`, `"\udc00\ud800"`, `"\uzzzz"`} {
		bad := strings.Replace(string(data), `"error":"safe"`, `"error":`+value, 1)
		_, err := decodeSnapshot([]byte(bad))
		assertPrivateError(t, err)
	}

	for _, value := range []string{`"\ud83d\ude00"`, `"\\ud800"`, `"\ufffd"`} {
		good := strings.Replace(string(data), `"error":"safe"`, `"error":`+value, 1)
		if _, err := decodeSnapshot([]byte(good)); err != nil {
			t.Fatalf("rejected valid Unicode escape: %v", err)
		}
	}
}

func TestSnapshotEncodedLimits(t *testing.T) {
	data, err := encodeSnapshot(NewSnapshot())
	if err != nil {
		t.Fatal(err)
	}

	padded := append(data, bytes.Repeat([]byte{' '}, MaxSnapshotSize-len(data))...)
	if _, err := decodeSnapshot(padded); err != nil {
		t.Fatal("rejected an input at the byte limit")
	}

	_, err = decodeSnapshot(append(padded, ' '))
	assertPrivateError(t, err)

	item, err := json.Marshal(testJob(t, t.TempDir(), 1))
	if err != nil {
		t.Fatal(err)
	}

	tooMany := `{"version":1,"parallel":1,"next_order":300,"jobs":[` + strings.Repeat(string(item)+",", MaxJobs) + string(item) + `]}`
	_, err = decodeSnapshot([]byte(tooMany))
	assertPrivateError(t, err)
	if !strings.Contains(err.Error(), "too many jobs") {
		t.Fatal("decoder did not stop at the job-count bound")
	}
}
