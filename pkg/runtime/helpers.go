package runtime

import "strings"

// shellQuote escapes a string for safe use in sh -c.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
