package metrics

import "time"

// ParseDuration parses soak duration/interval strings such as 30m, 4h, 90s.
func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
