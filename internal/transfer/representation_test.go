package transfer

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestRepresentationSnapshot(t *testing.T) {
	source, err := parseURL("https://example.org/file?token=private")
	if err != nil {
		t.Fatal(err)
	}

	resp := rangeResponse(t)
	id, ok := selectRepresentation(resp, source, 10)
	if !ok {
		t.Fatal("did not select an eligible representation")
	}

	before := id.metadata()
	resp.Header.Set("ETag", `"other"`)
	resp.Request.URL.Path = "/changed"
	resp.ContentLength = 100
	source.RawQuery = "token=changed"
	copy := id.metadata()
	copy.ETag = `"mutated"`
	copy.Length = 100

	after := id.metadata()
	if after.SourceID != before.SourceID || after.EffectiveID != before.EffectiveID ||
		after.Length != 10 || after.ETag != `"v1"` || after.Encoding != "identity" {
		t.Fatalf("selected identity was mutable: %+v", after)
	}

	if strings.Contains(after.SourceID, "private") || strings.Contains(after.EffectiveID, "private") {
		t.Fatal("identity metadata retained a URL secret")
	}
}

func TestRepresentationRequests(t *testing.T) {
	resp := rangeResponse(t)
	source := resp.Request.URL
	id, ok := selectRepresentation(resp, source, 10)
	if !ok {
		t.Fatal("did not select representation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := id.request(ctx, source, ranges.Span{Start: 3, End: 6})
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodGet || req.Header.Get("Range") != "bytes=3-5" || req.Header.Get("If-Range") != `"v1"` ||
		req.Header.Get("Accept-Encoding") != "identity" || req.Header.Get("User-Agent") != "yoinkdl" || req.Context() != ctx {
		t.Fatalf("invalid conditional range request: %+v", req)
	}

	for _, span := range []ranges.Span{{Start: -1, End: 2}, {Start: 1, End: 1}, {Start: 1, End: 11}} {
		if _, err := id.request(ctx, source, span); err == nil {
			t.Fatalf("accepted invalid interval: %v", span)
		}
	}

	other := *source
	other.RawQuery = "token=other-secret"
	if _, err := id.request(ctx, &other, ranges.Span{End: 1}); err == nil || strings.Contains(err.Error(), "other-secret") {
		t.Fatalf("missing or unsafe source mismatch: %v", err)
	}

	if _, err := (representation{}).request(ctx, source, ranges.Span{End: 1}); err == nil {
		t.Fatal("accepted an unselected representation")
	}
}

func TestRepresentationEligibility(t *testing.T) {
	for _, mode := range []string{"unknown", "empty", "missing", "weak", "duplicate", "encoded", "duplicate encoding"} {
		t.Run(mode, func(t *testing.T) {
			resp := rangeResponse(t)
			length := int64(10)

			switch mode {
			case "unknown":
				length = -1
			case "empty":
				length = 0
			case "missing":
				resp.Header.Del("ETag")
			case "weak":
				resp.Header.Set("ETag", `W/"v1"`)
			case "duplicate":
				resp.Header.Add("ETag", `"v1"`)
			case "encoded":
				resp.Header.Set("Content-Encoding", "gzip")
			case "duplicate encoding":
				resp.Header["Content-Encoding"] = []string{"identity", "identity"}
			}

			if _, ok := selectRepresentation(resp, resp.Request.URL, length); ok {
				t.Fatal("selected an ineligible representation")
			}
		})
	}
}

func TestRepresentationResponseValidation(t *testing.T) {
	for _, mode := range []string{
		"valid", "chunked", "status", "etag", "missing etag", "duplicate etag", "encoding", "redirect",
		"start", "end", "total", "unknown total", "missing range", "duplicate range", "length", "overflow",
	} {
		t.Run(mode, func(t *testing.T) {
			resp := rangeResponse(t)
			id, _ := selectRepresentation(resp, resp.Request.URL, 10)

			switch mode {
			case "chunked":
				resp.ContentLength = -1
			case "status":
				resp.StatusCode = http.StatusOK
			case "etag":
				resp.Header.Set("ETag", `"changed"`)
			case "missing etag":
				resp.Header.Del("ETag")
			case "duplicate etag":
				resp.Header.Add("ETag", `"v1"`)
			case "encoding":
				resp.Header.Set("Content-Encoding", "gzip")
			case "redirect":
				resp.Request.URL.RawQuery = "token=secret"
			case "start":
				resp.Header.Set("Content-Range", "bytes 2-5/10")
			case "end":
				resp.Header.Set("Content-Range", "bytes 3-6/10")
			case "total":
				resp.Header.Set("Content-Range", "bytes 3-5/11")
			case "unknown total":
				resp.Header.Set("Content-Range", "bytes 3-5/*")
			case "missing range":
				resp.Header.Del("Content-Range")
			case "duplicate range":
				resp.Header.Add("Content-Range", "bytes 3-5/10")
			case "length":
				resp.ContentLength = 2
			case "overflow":
				resp.Header.Set("Content-Range", "bytes 3-9223372036854775808/10")
			}

			err := id.validate(resp, ranges.Span{Start: 3, End: 6})
			valid := mode == "valid" || mode == "chunked"
			if (err == nil) != valid || (err != nil && strings.Contains(err.Error(), "secret")) {
				t.Fatalf("valid = %v, error = %v", valid, err)
			}
		})
	}

	resp := rangeResponse(t)
	id, _ := selectRepresentation(resp, resp.Request.URL, math.MaxInt64)
	resp.ContentLength = 1
	resp.Header.Set("Content-Range", "bytes 9223372036854775806-9223372036854775806/9223372036854775807")
	if err := id.validate(resp, ranges.Span{Start: math.MaxInt64 - 1, End: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
}

func rangeResponse(t *testing.T) *http.Response {
	t.Helper()
	u, err := url.Parse("https://cdn.example.org/file")
	if err != nil {
		t.Fatal(err)
	}

	return &http.Response{
		StatusCode: http.StatusPartialContent, ContentLength: 3,
		Request: &http.Request{URL: u},
		Header:  http.Header{"Etag": {`"v1"`}, "Content-Range": {"bytes 3-5/10"}},
	}
}
