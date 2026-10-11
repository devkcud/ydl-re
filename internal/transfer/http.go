package transfer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errRedirectLimit = errors.New("redirect limit exceeded")
var errRedirectTarget = errors.New("redirect target must be HTTP(S) without URL credentials")
var errDowngrade = errors.New("refusing an HTTPS to HTTP redirect")

func newClient(timeout time.Duration, limit *limiter) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	transport.DisableKeepAlives = true
	transport.DialContext = (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = timeout
	transport.ResponseHeaderTimeout = timeout
	transport.MaxResponseHeaderBytes = 1 << 20
	return &http.Client{
		Transport:     &bodyTransport{next: transport, timeout: timeout, limit: limit},
		CheckRedirect: checkRedirect,
	}
}

func validateFullResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d %s; expected a complete 200 response", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if len(resp.Header.Values("Content-Range")) != 0 {
		return errors.New("complete 200 responses must not include Content-Range")
	}

	return nil
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errRedirectLimit
	}

	if req.URL.User != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
		return errRedirectTarget
	}

	prev := via[len(via)-1].URL
	if prev.Scheme == "https" && req.URL.Scheme == "http" {
		return errDowngrade
	}

	first := via[0].URL
	if prev.Scheme != req.URL.Scheme || !strings.EqualFold(prev.Host, req.URL.Host) ||
		first.Scheme != req.URL.Scheme || !strings.EqualFold(first.Host, req.URL.Host) {
		for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
			req.Header.Del(key)
		}
	}

	return nil
}

func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("provide an absolute HTTP or HTTPS URL")
	}

	if u.User != nil {
		return nil, errors.New("credentials in URLs are not supported")
	}

	u.Fragment = ""
	return u, nil
}

func requestError(err error) error {
	for _, known := range []error{errRedirectLimit, errRedirectTarget, errDowngrade} {
		if errors.Is(err, known) {
			return known
		}
	}

	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return errors.New("TLS certificate verification failed")
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("network timeout during connection, TLS handshake, or response headers")
	}

	return errors.New("HTTP request failed; check connectivity, proxy settings, and redirect targets")
}
