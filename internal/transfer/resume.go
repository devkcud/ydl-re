package transfer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func fingerprint(u *url.URL) string {
	copy := *u
	copy.Fragment = ""
	copy.RawFragment = ""

	return fmt.Sprintf("%x", sha256.Sum256([]byte(copy.String())))
}

func responseMetadata(resp *http.Response, source *url.URL) storage.Manifest {
	encoding := "encoded"
	values := resp.Header.Values("Content-Encoding")

	if len(values) == 0 || (len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity")) {
		encoding = "identity"
	}

	tag := ""
	values = resp.Header.Values("ETag")

	if len(values) == 1 && storage.StrongETag(values[0]) {
		tag = values[0]
	}

	return storage.Manifest{
		SourceID:    fingerprint(source),
		EffectiveID: fingerprint(resp.Request.URL),
		Length:      resp.ContentLength,
		ETag:        tag,
		Encoding:    encoding,
	}
}

func validateResume(resp *http.Response, saved storage.Manifest) error {
	status := http.StatusOK
	if saved.Completed > 0 {
		status = http.StatusPartialContent
	}

	if resp.StatusCode != status {
		return fmt.Errorf("server did not honor resume (HTTP %d); existing state is preserved", resp.StatusCode)
	}

	current := responseMetadata(resp, resp.Request.URL)
	if current.ETag != saved.ETag || current.Encoding != "identity" || current.EffectiveID != saved.EffectiveID {
		return errors.New("remote version, encoding, or redirect target changed; existing state is preserved")
	}

	if saved.Completed == 0 {
		if resp.ContentLength != saved.Length {
			return errors.New("remote length changed; existing state is preserved")
		}

		return nil
	}

	return validateRangeBounds(resp, ranges.Span{Start: saved.Completed, End: saved.Length}, saved.Length)
}

func parseRange(value string) (int64, int64, int64, error) {
	value, ok := strings.CutPrefix(value, "bytes ")
	if !ok {
		return 0, 0, 0, errors.New("invalid range unit")
	}

	interval, total, ok := strings.Cut(value, "/")
	start, end, bounds := strings.Cut(interval, "-")
	if !ok || !bounds {
		return 0, 0, 0, errors.New("invalid range syntax")
	}

	var values [3]int64

	for i, part := range []string{start, end, total} {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return 0, 0, 0, errors.New("invalid range number")
		}

		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return 0, 0, 0, errors.New("range number overflow")
		}

		values[i] = n
	}

	return values[0], values[1], values[2], nil
}

func reprobe(ctx context.Context, client *http.Client, previous *http.Request) error {
	probe := previous.Clone(ctx)
	probe.Header.Del("Range")
	probe.Header.Del("If-Range")
	probe.Header.Del("If-Match")
	resp, err := client.Do(probe)

	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}

		return errors.New("range was rejected and re-probing failed; existing state is preserved")
	}
	resp.Body.Close()

	return errors.New("range was rejected; resource re-probed without inferring completion; existing state is preserved")
}
