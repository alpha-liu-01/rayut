package rules

import "strings"

// Classify reports whether one `ip rule` line belongs to the mihomo TUN
// priorities used on this device. Other priorities are ignored. A matching
// priority with an unrecognized selector is unexpected and must not be deleted.
func Classify(line string) (priority string, kind string) {
	line = strings.TrimSpace(line)
	priority, rest, ok := strings.Cut(line, ":")
	if !ok || !ownedPriority(priority) {
		return "", "ignore"
	}
	rest = strings.TrimSpace(rest)
	if strings.Contains(rest, "lookup 2022") ||
		strings.Contains(rest, "iif Meta") ||
		strings.Contains(rest, "suppress_prefixlength 0") ||
		strings.Contains(rest, "goto 9010") ||
		rest == "from all nop" {
		return priority, "owned"
	}
	return priority, "unexpected"
}

func ownedPriority(priority string) bool {
	switch priority {
	case "9000", "9001", "9002", "9010":
		return true
	default:
		return false
	}
}
