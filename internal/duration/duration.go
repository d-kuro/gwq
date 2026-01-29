// Package duration provides parsing for duration strings with day notation support.
package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// dayRegex matches day notation like "7d", "30d", etc.
var dayRegex = regexp.MustCompile(`^(\d+)d$`)

// Parse parses a duration string that supports both standard Go durations
// (e.g., "1h", "30m", "2h30m") and day notation (e.g., "1d", "7d", "30d").
func Parse(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Check for day notation first
	if matches := dayRegex.FindStringSubmatch(s); len(matches) == 2 {
		days, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("invalid day value: %w", err)
		}
		if days <= 0 {
			return 0, fmt.Errorf("days must be positive")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Fall back to standard Go duration parsing
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %w", err)
	}

	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	return d, nil
}
