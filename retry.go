package ipcheck

import (
	"context"
	"net/http"
	"time"
)

// retryRequest intentionally ignores any server-provided retry delay such as
// Retry-After. We keep retry pacing fully controlled by local config so a
// throttled endpoint cannot stretch ip-check runs indefinitely.
func retryRequest(ctx context.Context, maxRetry int, backoff float64, fn func() (*http.Response, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)
	attempts := maxRetry + 1
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		resp, err = fn()
		if err == nil {
			return resp, nil
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if i == attempts-1 {
			break
		}
		// Match ip-check2 behavior: retry wait time only follows local
		// backoff_factor and never any server response header.
		sleep := time.Duration(float64(time.Second) * backoff * float64(i+1))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(sleep):
		}
	}
	return nil, err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
