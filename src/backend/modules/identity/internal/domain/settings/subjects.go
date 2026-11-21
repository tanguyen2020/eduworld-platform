package settings

// KeyEnabledSubjects is the school setting enabled_subject_codes (F015 T054, F071 Câu hỏi mở F071-1): the subject
// codes the school keeps on; absent or null = every subject on (the default). Module game hides the learning
// areas of the other subjects (F071-AC-03) through school.updated settings_subset.
const KeyEnabledSubjects = "enabled_subject_codes"

// Bounds of enabled_subject_codes (ADR-0041: a code is at most 64 characters).
const (
	MaxEnabledSubjects  = 50
	MaxSubjectCodeChars = 64
)

// ValidateEnabledSubjects checks an enabled_subject_codes list: at least one subject (null turns every subject
// back on; an empty list would hide every learning area), distinct, non-blank codes of at most 64 characters.
// The codes are not checked against the subject catalog of module learning: an unknown code turns nothing on.
func ValidateEnabledSubjects(codes []string) error {
	if len(codes) == 0 || len(codes) > MaxEnabledSubjects {
		return ErrInvalid{KeyEnabledSubjects, "1..50 subject codes or null"}
	}
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if c == "" || len([]rune(c)) > MaxSubjectCodeChars {
			return ErrInvalid{KeyEnabledSubjects, "subject code of 1..64 characters"}
		}
		if seen[c] {
			return ErrInvalid{KeyEnabledSubjects, "duplicate subject code"}
		}
		seen[c] = true
	}
	return nil
}
