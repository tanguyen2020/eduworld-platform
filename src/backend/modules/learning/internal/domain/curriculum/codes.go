package curriculum

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Limits of the curriculum fields (data-model §1, ADR-0041).
const (
	MaxCodeLen        = 64
	MaxNameLen        = 255
	MaxDescriptionLen = 1000
	MaxYccdRefLen     = 64
	MaxChangeNoteLen  = 1000
	MaxNameJSONBytes  = 2048
	MaxPrerequisites  = 20
	MaxCompetencies   = 20
)

var (
	curriculumCodeRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
	subjectCodeRe    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	topicCodeRe      = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.]{0,63}$`)
	versionLabelRe   = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}$`)
	pageRangeRe      = regexp.MustCompile(`^[0-9]{1,4}(-[0-9]{1,4})?$`)
	// LO code <SUBJECT>.G<grade>.<STRAND>.<SKILL>[.<n>] (CURRICULUM_MODEL §4, data-model §3).
	loCodeRe = regexp.MustCompile(`^([A-Z]{2,8})\.G([1-9])\.([A-Z]{1,6})\.([A-Z][A-Z0-9_]{0,40})(\.[0-9]{1,2})?$`)
)

func invalid(field, reason string) contentmodel.ErrInvalid {
	return contentmodel.ErrInvalid{Field: field, Reason: reason}
}

// CheckCurriculumCode validates an UPPER_SNAKE curriculum (or competency, textbook series) code.
func CheckCurriculumCode(field, code string) error {
	if !curriculumCodeRe.MatchString(code) {
		return invalid(field, "must be UPPER_SNAKE, 2–64 characters")
	}
	return nil
}

// CheckSubjectCode validates a subject code of the catalog (e.g. math).
func CheckSubjectCode(code string) error {
	if !subjectCodeRe.MatchString(code) {
		return invalid("code", "must be lower_snake, 2–64 characters")
	}
	return nil
}

// CheckTopicCode validates a topic code (unique within a curriculum version).
func CheckTopicCode(code string) error {
	if !topicCodeRe.MatchString(code) {
		return invalid("code", "must be 1–64 characters of A–Z, 0–9, _ and .")
	}
	return nil
}

// CheckVersionLabel validates MAJOR.MINOR.
func CheckVersionLabel(label string) error {
	if !versionLabelRe.MatchString(label) {
		return invalid("version_label", "must be MAJOR.MINOR")
	}
	return nil
}

// CheckPageRange validates a page range of a textbook section (e.g. 12 or 12-15).
func CheckPageRange(r string) error {
	if !pageRangeRe.MatchString(r) {
		return invalid("page_range", "must be a page or a page range such as 12-15")
	}
	if a, b, ok := strings.Cut(r, "-"); ok {
		x, _ := strconv.Atoi(a)
		y, _ := strconv.Atoi(b)
		if y < x {
			return invalid("page_range", "the last page is before the first")
		}
	}
	return nil
}

// LoCode is a parsed learning objective code.
type LoCode struct {
	Subject string
	Grade   int
	Strand  string
	Skill   string
	Detail  string // ".n" or empty
}

// ParseLoCode parses and validates a learning objective code (F021-AC-03).
func ParseLoCode(code string) (LoCode, error) {
	m := loCodeRe.FindStringSubmatch(code)
	if m == nil {
		return LoCode{}, invalid("code", "must follow <SUBJECT>.G<grade>.<STRAND>.<SKILL>[.<n>]")
	}
	g, _ := strconv.Atoi(m[2])
	return LoCode{Subject: m[1], Grade: g, Strand: m[3], Skill: m[4], Detail: m[5]}, nil
}

// CheckLoCodeForGrade validates the code and that its G<grade> is the grade of the topic holding the LO.
func CheckLoCodeForGrade(code string, grade int) error {
	c, err := ParseLoCode(code)
	if err != nil {
		return err
	}
	if c.Grade != grade {
		return invalid("code", "G"+strconv.Itoa(c.Grade)+" does not match the grade "+strconv.Itoa(grade)+" of the topic")
	}
	return nil
}

// CheckName validates a required short text (topic name).
func CheckName(field, s string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	if n == 0 {
		return contentmodel.ErrInvalid{Field: field, Reason: "is required"}
	}
	if utf8.RuneCountInString(s) > MaxNameLen {
		return invalid(field, "is longer than 255 characters")
	}
	return nil
}

// CheckLearningName validates an i18n name (LearningName): vi or en, each 1–255 characters, nothing else.
func CheckLearningName(field string, name map[string]string) error {
	if len(name) == 0 {
		return invalid(field, "needs at least one language")
	}
	for lang, v := range name {
		if lang != string(contentmodel.LangVI) && lang != string(contentmodel.LangEN) {
			return invalid(field, "languages are vi and en")
		}
		if n := utf8.RuneCountInString(v); n == 0 || n > MaxNameLen {
			return invalid(field+"."+lang, "must be 1–255 characters")
		}
	}
	return nil
}
