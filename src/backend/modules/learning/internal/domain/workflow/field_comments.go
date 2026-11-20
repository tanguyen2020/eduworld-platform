package workflow

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Limits of the per-field comments of a review (OQ F054-1, owner 2026-11-20; transitionContentVersion.field_comments).
const (
	MaxFieldComments      = 50
	MaxFieldCommentPath   = 256
	MaxFieldCommentLength = 1000
)

// FieldComment is a reviewer comment on one field of the version (path is a JSON pointer, RFC 6901).
type FieldComment struct {
	Path    string `json:"path"`
	Comment string `json:"comment"`
}

// CheckFieldComments validates the field comments of a transition: only a reject carries them (the workflow has no
// request_changes action), at most 50, path a JSON pointer of at most 256 characters, comment 1..1000 characters.
func CheckFieldComments(a contentmodel.Action, cs []FieldComment) contentmodel.Violations {
	if len(cs) == 0 {
		return nil
	}
	if a != contentmodel.ActionReject {
		return contentmodel.Violations{{Field: "/field_comments", Reason: "is accepted with the reject action only"}}
	}
	if len(cs) > MaxFieldComments {
		return contentmodel.Violations{{Field: "/field_comments", Reason: "has more than 50 comments"}}
	}
	var vs contentmodel.Violations
	for i, c := range cs {
		at := "/field_comments/" + strconv.Itoa(i)
		if !jsonPointer(c.Path) {
			vs = append(vs, contentmodel.Violation{Field: at + "/path", Reason: "is not a JSON pointer of at most 256 characters"})
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(c.Comment)); n == 0 || utf8.RuneCountInString(c.Comment) > MaxFieldCommentLength {
			vs = append(vs, contentmodel.Violation{Field: at + "/comment", Required: n == 0, Reason: "must have 1 to 1000 characters"})
		}
	}
	return vs
}

// jsonPointer accepts "" or "/…" with valid escapes (~0, ~1), no control characters, at most 256 characters.
func jsonPointer(p string) bool {
	if utf8.RuneCountInString(p) > MaxFieldCommentPath || (p != "" && p[0] != '/') {
		return false
	}
	for i, r := range p {
		if unicode.IsControl(r) {
			return false
		}
		if r == '~' && (i+1 >= len(p) || (p[i+1] != '0' && p[i+1] != '1')) {
			return false
		}
	}
	return true
}
