package transfer

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

type representation struct {
	sourceID    string
	effectiveID string
	length      int64
	etag        string
}

func selectRepresentation(resp *http.Response, source *url.URL, length int64) (representation, bool) {
	m := responseMetadata(resp, source)
	if length <= 0 || m.Encoding != "identity" || !storage.StrongETag(m.ETag) {
		return representation{}, false
	}

	return representation{
		sourceID: m.SourceID, effectiveID: m.EffectiveID, length: length, etag: m.ETag,
	}, true
}

func (r representation) metadata() storage.Manifest {
	return storage.Manifest{
		SourceID: r.sourceID, EffectiveID: r.effectiveID, Length: r.length,
		ETag: r.etag, Encoding: "identity",
	}
}

func (r representation) request(ctx context.Context, source *url.URL, span ranges.Span) (*http.Request, error) {
	if r.length <= 0 || !storage.StrongETag(r.etag) || fingerprint(source) != r.sourceID || span.End > r.length {
		return nil, errors.New("range request does not match the selected representation")
	}

	value, err := span.HTTPRange()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.String(), nil)
	if err != nil {
		return nil, errors.New("cannot construct range request")
	}

	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "yoinkdl")
	req.Header.Set("Range", value)
	req.Header.Set("If-Range", r.etag)
	return req, nil
}

func (r representation) validate(resp *http.Response, span ranges.Span) error {
	if resp.StatusCode != http.StatusPartialContent {
		return errors.New("server did not honor the requested range")
	}

	m := responseMetadata(resp, resp.Request.URL)
	if m.ETag != r.etag || !storage.StrongETag(r.etag) || m.Encoding != "identity" || m.EffectiveID != r.effectiveID {
		return errors.New("remote version, encoding, or redirect target changed")
	}

	return validateRangeBounds(resp, span, r.length)
}

func validateRangeBounds(resp *http.Response, span ranges.Span, length int64) error {
	if span.Start < 0 || span.End <= span.Start || span.End > length {
		return errors.New("invalid requested range bounds")
	}

	values := resp.Header.Values("Content-Range")
	if len(values) != 1 {
		return errors.New("range response must have exactly one Content-Range")
	}

	start, end, total, err := parseRange(values[0])
	if err != nil || start != span.Start || end != span.End-1 || total != length {
		return errors.New("range response has incompatible byte bounds")
	}

	if resp.ContentLength >= 0 && resp.ContentLength != span.End-span.Start {
		return errors.New("range response has an incompatible payload length")
	}

	return nil
}
