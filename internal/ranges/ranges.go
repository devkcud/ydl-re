package ranges

import (
	"errors"
	"fmt"
)

const MinChunkSize int64 = 4 * 1024 * 1024
const MaxChunks = 256

type Span struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

func ChunkSize(length int64) (int64, error) {
	if length < 0 {
		return 0, errors.New("range planning requires a known nonnegative length")
	}

	size := length / MaxChunks
	if length%MaxChunks != 0 {
		size++
	}

	return max(MinChunkSize, size), nil
}

func Covered(length, size int64, done []Span) (int64, error) {
	if length < 0 || size <= 0 || len(done) > MaxChunks {
		return 0, errors.New("invalid range plan bounds")
	}

	count := length / size
	if length%size != 0 {
		count++
	}

	if count > MaxChunks {
		return 0, errors.New("range plan exceeds the chunk limit")
	}

	var covered, end int64

	for _, span := range done {
		if span.Start < end || span.Start < 0 || span.Start >= length || span.Start%size != 0 ||
			span.End != span.Start+min(size, length-span.Start) {
			return 0, errors.New("completed ranges must be sorted, disjoint whole chunks")
		}

		covered += span.End - span.Start
		end = span.End
	}

	return covered, nil
}

func Missing(length, size int64, done []Span) ([]Span, error) {
	if _, err := Covered(length, size, done); err != nil {
		return nil, err
	}

	var missing []Span
	next := 0

	for start := int64(0); start < length; {
		end := start + min(size, length-start)

		if next < len(done) && done[next].Start == start {
			next++
		} else {
			missing = append(missing, Span{Start: start, End: end})
		}

		start = end
	}

	return missing, nil
}

func (s Span) HTTPRange() (string, error) {
	if s.Start < 0 || s.End <= s.Start {
		return "", errors.New("HTTP range requires a nonempty nonnegative interval")
	}

	return fmt.Sprintf("bytes=%d-%d", s.Start, s.End-1), nil
}
