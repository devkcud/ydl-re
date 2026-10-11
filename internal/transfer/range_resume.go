package transfer

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func validateRangeResume(ctx context.Context, client *http.Client, source *url.URL, saved storage.Manifest,
	timeout time.Duration) (representation, error) {
	if saved.Version != storage.RangeManifestVersion || saved.State != storage.StatePartial ||
		!saved.Resumable() || saved.SourceID != fingerprint(source) || timeout <= 0 {
		return representation{}, errors.New("saved range identity cannot resume")
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	id := representation{
		sourceID: saved.SourceID, effectiveID: saved.EffectiveID, length: saved.Length, etag: saved.ETag,
	}
	var req *http.Request
	var err error
	size := int64(1)

	if saved.Length == 0 {
		size = 0
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, source.String(), nil)
		if err != nil {
			return representation{}, errors.New("cannot construct empty range resume request")
		}

		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("User-Agent", "yoinkdl")
		req.Header.Set("If-Match", saved.ETag)
	} else {
		req, err = id.request(ctx, source, ranges.Span{End: 1})
		if err != nil {
			return representation{}, err
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return representation{}, cause
		}

		return representation{}, requestError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		resp.Body.Close()
		return representation{}, reprobe(ctx, client, req)
	}

	if saved.Length == 0 {
		if err := validateResume(resp, saved); err != nil {
			return representation{}, err
		}

		if len(resp.Header.Values("Content-Range")) != 0 {
			return representation{}, errors.New("empty range resume requires a complete response")
		}
	} else if err := id.validate(resp, ranges.Span{End: 1}); err != nil {
		return representation{}, err
	}

	body := &probeBody{ReadCloser: resp.Body, ctx: ctx, cancel: cancel, timeout: timeout}
	if err := readProbeBody(body, size); err != nil {
		return representation{}, err
	}

	if err := ctx.Err(); err != nil {
		return representation{}, err
	}

	return id, nil
}
