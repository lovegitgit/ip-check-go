package ipcheck

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

type transportBody struct {
	io.ReadCloser
	transport *http.Transport
}

func (b *transportBody) Close() error {
	err := b.ReadCloser.Close()
	if b.transport != nil {
		b.transport.CloseIdleConnections()
	}
	return err
}

type timeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (c *timeoutConn) Read(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Read(p)
}

func (c *timeoutConn) Write(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Write(p)
}

func pinnedTransport(targetIP string, port int, serverName string, timeout time.Duration) *http.Transport {
	dialer := &net.Dialer{Timeout: timeout}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(targetIP, fmt.Sprint(port)))
			if err != nil {
				return nil, err
			}
			return &timeoutConn{Conn: conn, timeout: timeout}, nil
		},
		TLSClientConfig: &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: true,
		},
		DisableKeepAlives:    true,
		TLSHandshakeTimeout:   timeout,
		DisableCompression:    true,
		ResponseHeaderTimeout: timeout,
		ForceAttemptHTTP2:     false,
	}
}

func doPinnedGET(ctx context.Context, targetIP string, port int, rawURL string, hostOverride string, timeout time.Duration, userAgent string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if hostOverride != "" {
		req.Host = hostOverride
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	transport := pinnedTransport(targetIP, port, req.URL.Hostname(), timeout)
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme == "" {
				base := via[len(via)-1].URL
				req.URL = base.ResolveReference(req.URL)
			}
			req.Host = req.URL.Hostname()
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	if resp.Body != nil {
		resp.Body = &transportBody{ReadCloser: resp.Body, transport: transport}
	} else {
		transport.CloseIdleConnections()
	}
	return resp, nil
}

func newTimeoutHTTPClient(proxy string, timeout time.Duration) (*http.Client, error) {
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &timeoutConn{Conn: conn, timeout: timeout}, nil
		},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
	}
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(pu)
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

func readLimitedBody(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}

func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
