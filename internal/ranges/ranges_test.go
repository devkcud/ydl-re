package ranges

import (
	"math"
	"reflect"
	"testing"
)

func TestMissing(t *testing.T) {
	cases := []struct {
		name   string
		length int64
		size   int64
		done   []Span
		want   []Span
	}{
		{"empty", 0, 4, nil, nil},
		{"one byte", 1, 4, nil, []Span{{0, 1}}},
		{"uneven", 10, 4, nil, []Span{{0, 4}, {4, 8}, {8, 10}}},
		{"holes", 14, 4, []Span{{0, 4}, {8, 12}}, []Span{{4, 8}, {12, 14}}},
		{"adjacent", 10, 4, []Span{{0, 4}, {4, 8}}, []Span{{8, 10}}},
		{"complete", 10, 4, []Span{{0, 4}, {4, 8}, {8, 10}}, nil},
		{"maximum offset", math.MaxInt64, math.MaxInt64 - 1, []Span{{0, math.MaxInt64 - 1}}, []Span{{math.MaxInt64 - 1, math.MaxInt64}}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := Missing(test.length, test.size, test.done)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("missing = %v, error = %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestInvalidCoverage(t *testing.T) {
	cases := []struct {
		name   string
		length int64
		size   int64
		done   []Span
	}{
		{"unknown", -1, 4, nil},
		{"zero chunk", 10, 0, nil},
		{"negative chunk", 10, -1, nil},
		{"too many chunks", MaxChunks + 1, 1, nil},
		{"negative", 10, 4, []Span{{-4, 0}}},
		{"empty", 10, 4, []Span{{4, 4}}},
		{"reversed", 10, 4, []Span{{4, 0}}},
		{"outside", 10, 4, []Span{{8, 12}}},
		{"unaligned", 10, 4, []Span{{1, 5}}},
		{"partial chunk", 10, 4, []Span{{0, 3}}},
		{"merged chunks", 10, 4, []Span{{0, 8}}},
		{"duplicate", 10, 4, []Span{{0, 4}, {0, 4}}},
		{"overlap", 10, 4, []Span{{0, 4}, {2, 6}}},
		{"unsorted", 10, 4, []Span{{4, 8}, {0, 4}}},
		{"empty object", 0, 4, []Span{{0, 1}}},
		{"overflow", math.MaxInt64, math.MaxInt64, []Span{{math.MaxInt64 - 1, math.MinInt64}}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got, err := Missing(test.length, test.size, test.done); err == nil || got != nil {
				t.Fatalf("accepted invalid coverage: %v, %v", got, err)
			}
		})
	}
}

func TestCoveragePartitions(t *testing.T) {
	for length := int64(0); length <= 24; length++ {
		for size := int64(3); size <= 8; size++ {
			plan, err := Missing(length, size, nil)
			if err != nil {
				t.Fatal(err)
			}

			for mask := 0; mask < 1<<len(plan); mask++ {
				var done []Span
				for i, span := range plan {
					if mask&(1<<i) != 0 {
						done = append(done, span)
					}
				}

				missing, err := Missing(length, size, done)
				if err != nil {
					t.Fatal(err)
				}

				bytes := make([]int, length)
				var saved int64

				for _, span := range done {
					saved += span.End - span.Start
				}

				got, err := Covered(length, size, done)
				if err != nil || got != saved {
					t.Fatalf("covered = %d, error = %v; want %d", got, err, saved)
				}

				for _, group := range [][]Span{done, missing} {
					for _, span := range group {
						for offset := span.Start; offset < span.End; offset++ {
							bytes[offset]++
						}
					}
				}

				for offset, count := range bytes {
					if count != 1 {
						t.Fatalf("length=%d size=%d done=%v: offset %d covered %d times", length, size, done, offset, count)
					}
				}
			}
		}
	}
}

func TestChunkSize(t *testing.T) {
	for _, length := range []int64{0, 1, MinChunkSize, MinChunkSize * MaxChunks, MinChunkSize*MaxChunks + 1, math.MaxInt64} {
		size, err := ChunkSize(length)
		if err != nil || size < MinChunkSize {
			t.Fatalf("chunk size = %d, error = %v", size, err)
		}

		plan, err := Missing(length, size, nil)
		if err != nil || len(plan) > MaxChunks {
			t.Fatalf("unbounded plan for %d: %d chunks, error = %v", length, len(plan), err)
		}

		covered, err := Covered(length, size, plan)
		if err != nil || covered != length {
			t.Fatalf("coverage = %d, error = %v; want %d", covered, err, length)
		}
	}

	if _, err := ChunkSize(-1); err == nil {
		t.Fatal("planned unknown length")
	}
}

func TestHTTPRange(t *testing.T) {
	for span, want := range map[Span]string{
		{0, 1}:                             "bytes=0-0",
		{1024, 2048}:                       "bytes=1024-2047",
		{math.MaxInt64 - 1, math.MaxInt64}: "bytes=9223372036854775806-9223372036854775806",
	} {
		got, err := span.HTTPRange()
		if err != nil || got != want {
			t.Fatalf("%v: range = %q, error = %v; want %q", span, got, err, want)
		}
	}

	for _, span := range []Span{{-1, 1}, {0, 0}, {2, 1}} {
		if _, err := span.HTTPRange(); err == nil {
			t.Fatalf("accepted invalid HTTP range: %v", span)
		}
	}
}
