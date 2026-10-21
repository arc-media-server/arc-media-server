package metadata

import (
	"net/http"
	"strconv"
	"time"
)

func retryStatus(status int) bool {
	return status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if raw := resp.Header.Get("Retry-After"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
			return time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(raw); err == nil {
			return max(time.Until(date), 0)
		}
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
