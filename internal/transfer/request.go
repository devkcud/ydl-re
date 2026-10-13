package transfer

import (
	"errors"
	"fmt"
	"net/url"
)

func Normalize(req Request) (Request, error) {
	req, _, err := normalizeRequest(req)
	return req, err
}

func normalizeRequest(req Request) (Request, *url.URL, error) {
	u, err := parseURL(req.URL)
	if err != nil {
		return Request{}, nil, err
	}

	if req.Timeout < 0 {
		return Request{}, nil, errors.New("timeout must be positive")
	}

	if req.Timeout == 0 {
		req.Timeout = DefaultTimeout
	}

	if req.Connections < 0 || req.Connections > MaxConnections {
		return Request{}, nil, fmt.Errorf("connections must be between 1 and %d", MaxConnections)
	}

	if req.Connections == 0 {
		req.Connections = DefaultConnections
	}

	retries := DefaultRetries
	if req.Retries != nil {
		retries = *req.Retries
	}

	if retries < 0 || retries > MaxRetries {
		return Request{}, nil, fmt.Errorf("retries must be between 0 and %d", MaxRetries)
	}

	if req.Limit < 0 {
		return Request{}, nil, errors.New("limit must not be negative")
	}

	if req.Output == "" {
		req.Output = filename(u)
	}

	req.URL = u.String()
	req.Retries = &retries
	return req, u, nil
}
