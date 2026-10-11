package transfer

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"yoinkdl/internal/ranges"
	"yoinkdl/internal/storage"
)

func validateRangeResume(ctx context.Context, client *http.Client, source *url.URL, saved storage.Manifest,
	policy retryPolicy) (representation, error) {
	var id representation
	err := policy.run(ctx, func(int) error {
		var err error
		id, err = rangeResumeAttempt(ctx, client, source, saved)
		return err
	})
	return id, err
}

func rangeResumeAttempt(ctx context.Context, client *http.Client, source *url.URL, saved storage.Manifest) (representation, error) {
	if saved.Version != storage.RangeManifestVersion || saved.State != storage.StatePartial ||
		!saved.Resumable() || saved.SourceID != fingerprint(source) {
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

	resp, err := requestOnce(ctx, client, req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return representation{}, cause
		}

		return representation{}, err
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

	if err := readProbeBody(resp.Body, size); err != nil {
		return representation{}, err
	}

	if err := ctx.Err(); err != nil {
		return representation{}, err
	}

	return id, nil
}
