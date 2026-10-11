package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"yoinkdl/internal/ranges"
)

type probeResult struct {
	identity representation
	stream   *http.Response
}

func probe(ctx context.Context, client *http.Client, source *url.URL, policy retryPolicy) (probeResult, error) {
	var result probeResult
	err := policy.run(ctx, func(int) error {
		var err error
		result, err = probeAttempt(ctx, client, source)
		return err
	})
	if err != nil || result.stream != nil || result.identity.length > 0 {
		return result, err
	}

	var resp *http.Response
	err = policy.run(ctx, func(int) error {
		var err error
		resp, err = probeRequest(ctx, client, source, false)
		return err
	})
	if err != nil {
		return probeResult{}, err
	}

	return sequentialProbe(resp)
}

func probeAttempt(ctx context.Context, client *http.Client, source *url.URL) (probeResult, error) {
	resp, err := probeRequest(ctx, client, source, true)
	if err != nil {
		return probeResult{}, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return sequentialProbe(resp)

	case http.StatusPartialContent:
		length, err := probeLength(resp)
		if err != nil {
			resp.Body.Close()
			return probeResult{}, err
		}

		err = readProbeBody(resp.Body, 1)
		closeErr := resp.Body.Close()
		if err != nil {
			return probeResult{}, err
		}

		if closeErr != nil {
			return probeResult{}, errors.New("cannot close range probe response")
		}

		if id, ok := selectRepresentation(resp, source, length); ok {
			return probeResult{identity: id}, nil
		}

	case http.StatusRequestedRangeNotSatisfiable:
		resp.Body.Close()

	default:
		resp.Body.Close()
		return probeResult{}, fmt.Errorf("HTTP %d %s during capability probe", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return probeResult{}, nil
}

func readProbeBody(body io.Reader, size int64) error {
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if int64(len(data)) > size {
		return errors.New("range probe body has an incompatible length")
	}
	if err != nil {
		return bodyFailure(err)
	}
	if int64(len(data)) < size {
		return bodyFailure(io.ErrUnexpectedEOF)
	}

	return nil
}

func probeLength(resp *http.Response) (int64, error) {
	values := resp.Header.Values("Content-Range")
	if len(values) != 1 {
		return 0, errors.New("range probe must have exactly one Content-Range")
	}

	value := values[0]
	unknown := strings.HasSuffix(value, "/*")
	if unknown {
		value = strings.TrimSuffix(value, "*") + "1"
	}

	start, end, total, err := parseRange(value)
	if err != nil || start != 0 || end != 0 || total <= 0 {
		return 0, errors.New("range probe has invalid byte bounds")
	}

	if unknown {
		if resp.ContentLength >= 0 && resp.ContentLength != 1 {
			return 0, errors.New("range probe has an incompatible payload length")
		}

		return -1, nil
	}

	if err := validateRangeBounds(resp, ranges.Span{End: 1}, total); err != nil {
		return 0, err
	}

	return total, nil
}

func sequentialProbe(resp *http.Response) (probeResult, error) {
	if resp.StatusCode != http.StatusOK || len(resp.Header.Values("Content-Range")) != 0 {
		resp.Body.Close()
		return probeResult{}, errors.New("sequential fallback requires a complete 200 response without Content-Range")
	}

	return probeResult{stream: resp}, nil
}

func probeRequest(ctx context.Context, client *http.Client, source *url.URL, ranged bool) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.String(), nil)
	if err != nil {
		return nil, errors.New("cannot construct capability request")
	}

	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "yoinkdl")
	if ranged {
		req.Header.Set("Range", "bytes=0-0")
	}

	return requestOnce(ctx, client, req)
}
