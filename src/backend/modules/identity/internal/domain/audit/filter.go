package audit

import (
	"errors"
	"regexp"
)

// Limits of the action_prefix filter of the audit queries (F059-3, OpenAPI AuditActionPrefix).
const (
	MaxActionPrefixes   = 10
	MaxActionPrefixLen  = 64
	minActionPrefixLen  = 2
	actionPrefixPattern = `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`
)

var prefixRe = regexp.MustCompile(actionPrefixPattern)

// ErrActionPrefix is an action_prefix list that is too long or has a malformed value.
var ErrActionPrefix = errors.New("audit: invalid action prefix")

// CheckActionPrefixes validates the action_prefix values: at most MaxActionPrefixes, each a dotted lower-case
// prefix of 2..MaxActionPrefixLen characters (letters, digits, underscore). A value matches an action equal to
// it or starting with "<value>." — never a plain substring, so "role" does not match "role_x.created".
func CheckActionPrefixes(prefixes []string) error {
	if len(prefixes) > MaxActionPrefixes {
		return ErrActionPrefix
	}
	for _, p := range prefixes {
		if len(p) < minActionPrefixLen || len(p) > MaxActionPrefixLen || !prefixRe.MatchString(p) {
			return ErrActionPrefix
		}
	}
	return nil
}

// MatchesPrefix reports whether action belongs to the group named by prefix (the rule of the SQL filter).
func MatchesPrefix(action, prefix string) bool {
	return action == prefix || (len(action) > len(prefix) && action[:len(prefix)] == prefix && action[len(prefix)] == '.')
}
