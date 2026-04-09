package ipcheck

import (
	"context"
	mrand "math/rand"
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
	rnd := mrand.New(mrand.NewSource(time.Now().UnixNano()))
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
		if backoff > 0 {
			jitterMax := time.Duration(float64(time.Second) * backoff / 2)
			if jitterMax < time.Nanosecond {
				jitterMax = time.Nanosecond
			}
			jitter := time.Duration(rnd.Int63n(int64(jitterMax)))
			sleep += jitter
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, ctx.Err()
		case <-timer.C:
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
