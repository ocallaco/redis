package internal

import (
	"time"

	"github.com/ocallaco/redis/v8/internal/rand"
)

func RetryBackoff(retry int, minBackoff, maxBackoff time.Duration) time.Duration {
	if retry < 0 {
		panic("not reached")
	}
	if minBackoff == 0 {
		return 0
	}
	// capping this to avoid d becoming a negative duration
	if retry > 10 {
		retry = 10
	}

	d := minBackoff << uint(retry)
	d = minBackoff + time.Duration(rand.Int63n(int64(d)))

	if d > maxBackoff || d < minBackoff {
		d = maxBackoff
	}

	return d
}
