package redact

import "regexp"

// Text removes credentials before a log line is stored or returned.
// Keys may remain. Values for tokens, passwords, UUIDs and Reality material do not.
func Text(line string) string {
	line = userinfoPattern.ReplaceAllString(line, "://[redacted]@")
	line = queryPattern.ReplaceAllString(line, "${1}[redacted]")
	line = assignPattern.ReplaceAllString(line, "${1}${2}${3}[redacted]")
	line = uuidPattern.ReplaceAllString(line, "[uuid]")
	line = bearerPattern.ReplaceAllString(line, "${1}[redacted]")
	return line
}

var (
	uuidPattern     = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	assignPattern   = regexp.MustCompile(`(?i)("?(?:password|passwd|token|secret|uuid|public-key|private-key|public_key|private_key|publickey|privatekey|short-id|short_id|shortid)"?)(\s*[:=]\s*)("?)([^"',\s}#]+)`)
	queryPattern    = regexp.MustCompile(`(?i)([?&](?:token|key|secret|password|uuid|access_token|pbk|sid)=)([^&#\s]+)`)
	bearerPattern   = regexp.MustCompile(`(?i)(Bearer\s+)\S+`)
	userinfoPattern = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
)
