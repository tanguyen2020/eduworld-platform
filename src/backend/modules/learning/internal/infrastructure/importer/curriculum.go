// Package importer reads and writes the curriculum files of module learning (F021 US5,
// specs/schemas/curriculum-import.schema.json): YAML (full tree) and CSV (one row per LO, no competencies). Parse
// reports structural problems row by row; business rules are checked by the import use case.
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// Codec implements port.CurriculumCodec.
type Codec struct{}

var _ port.CurriculumCodec = Codec{}

// Row error codes of the parser.
const (
	ErrInvalidFormat = "INVALID_FORMAT"
	ErrUnknownField  = "UNKNOWN_FIELD"
	ErrRequired      = "REQUIRED"
	ErrInvalid       = "INVALID"
)

type report struct{ errs []dto.ImportRowError }

func (r *report) add(row int, field, code, msg string) {
	if len(r.errs) < 500 {
		r.errs = append(r.errs, dto.ImportRowError{Row: max(row, 1), Field: field, Code: code, Message: msg})
	}
}

// Parse implements port.CurriculumCodec.
func (Codec) Parse(kind string, r io.Reader) (dto.CurriculumFile, []dto.ImportRowError) {
	switch kind {
	case port.KindCurriculumYAML:
		return parseYAML(r)
	case port.KindCurriculumCSV:
		return parseCSV(r)
	}
	return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Field: "kind", Code: ErrInvalid, Message: "unknown import kind"}}
}

// ---------- YAML

type mapping struct {
	node   *yaml.Node
	fields map[string]*yaml.Node
}

func asMapping(n *yaml.Node, field string, allowed []string, rep *report) (mapping, bool) {
	if n == nil || n.Kind != yaml.MappingNode {
		line := 1
		if n != nil {
			line = n.Line
		}
		rep.add(line, field, ErrInvalid, "must be a mapping")
		return mapping{}, false
	}
	m := mapping{node: n, fields: map[string]*yaml.Node{}}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		if !slices.Contains(allowed, k) {
			rep.add(n.Content[i].Line, prefixed(field, k), ErrUnknownField, "is not a field of the curriculum file")
			continue
		}
		m.fields[k] = n.Content[i+1]
	}
	return m, true
}

func prefixed(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return prefix + "." + k
}

func (m mapping) str(field, key string, required bool, maxLen int, rep *report) string {
	n, ok := m.fields[key]
	if !ok || n.Tag == "!!null" {
		if required {
			rep.add(m.node.Line, prefixed(field, key), ErrRequired, "is required")
		}
		return ""
	}
	if n.Kind != yaml.ScalarNode {
		rep.add(n.Line, prefixed(field, key), ErrInvalid, "must be a text")
		return ""
	}
	if len([]rune(n.Value)) > maxLen {
		rep.add(n.Line, prefixed(field, key), "MAX_LENGTH", "is longer than "+strconv.Itoa(maxLen)+" characters")
	}
	return n.Value
}

func (m mapping) integer(field, key string, required bool, rep *report) int {
	n, ok := m.fields[key]
	if !ok {
		if required {
			rep.add(m.node.Line, prefixed(field, key), ErrRequired, "is required")
		}
		return 0
	}
	v, err := strconv.Atoi(n.Value)
	if n.Kind != yaml.ScalarNode || err != nil {
		rep.add(n.Line, prefixed(field, key), ErrInvalid, "must be an integer")
		return 0
	}
	return v
}

func (m mapping) date(field, key string, rep *report) *time.Time {
	s := m.str(field, key, false, 10, rep)
	if s == "" {
		return nil
	}
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		rep.add(m.fields[key].Line, prefixed(field, key), ErrInvalid, "must be a date YYYY-MM-DD")
		return nil
	}
	return &d
}

func (m mapping) list(field, key string, rep *report) []*yaml.Node {
	n, ok := m.fields[key]
	if !ok || n.Tag == "!!null" {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		rep.add(n.Line, prefixed(field, key), ErrInvalid, "must be a list")
		return nil
	}
	return n.Content
}

func (m mapping) strings(field, key string, rep *report) []string {
	var out []string
	for _, n := range m.list(field, key, rep) {
		if n.Kind != yaml.ScalarNode || len(n.Value) > 64 {
			rep.add(n.Line, prefixed(field, key), ErrInvalid, "must be codes of at most 64 characters")
			continue
		}
		out = append(out, n.Value)
	}
	return out
}

func (m mapping) name(field, key string, rep *report) map[string]string {
	n, ok := m.fields[key]
	if !ok {
		rep.add(m.node.Line, prefixed(field, key), ErrRequired, "is required")
		return nil
	}
	nm, ok := asMapping(n, prefixed(field, key), []string{"vi", "en"}, rep)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for _, lang := range []string{"vi", "en"} {
		if s := nm.str(prefixed(field, key), lang, false, 255, rep); s != "" {
			out[lang] = s
		}
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseYAML(r io.Reader) (dto.CurriculumFile, []dto.ImportRowError) {
	rep := &report{}
	var root yaml.Node
	if err := yaml.NewDecoder(r).Decode(&root); err != nil {
		line := 1
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			fmt.Sscanf(err.Error(), "yaml: line %d", &line) //nolint:errcheck // best effort line of the syntax error
		}
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: max(line, 1), Code: ErrInvalidFormat, Message: "the file is not valid YAML"}}
	}
	if len(root.Content) == 0 {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: ErrInvalidFormat, Message: "the file is empty"}}
	}
	doc, ok := asMapping(root.Content[0], "", []string{"format_version", "curriculum", "version", "competencies", "subject_grades"}, rep)
	if !ok {
		return dto.CurriculumFile{}, rep.errs
	}
	f := dto.CurriculumFile{FormatVersion: doc.integer("", "format_version", true, rep)}
	if f.FormatVersion != 1 && len(rep.errs) == 0 {
		rep.add(doc.node.Line, "format_version", ErrInvalid, "must be 1")
	}
	if cn, ok := doc.fields["curriculum"]; !ok {
		rep.add(doc.node.Line, "curriculum", ErrRequired, "is required")
	} else if cm, ok := asMapping(cn, "curriculum", []string{"code", "name", "framework"}, rep); ok {
		f.Curriculum = dto.FileCurriculum{Code: cm.str("curriculum", "code", true, 64, rep), Name: cm.name("curriculum", "name", rep),
			Framework: optional(cm.str("curriculum", "framework", false, 64, rep)), Row: cn.Line}
	}
	if vn, ok := doc.fields["version"]; !ok {
		rep.add(doc.node.Line, "version", ErrRequired, "is required")
	} else if vm, ok := asMapping(vn, "version", []string{"version_label", "effective_from", "effective_to", "change_note", "content_language"}, rep); ok {
		f.Version = dto.FileVersion{Label: vm.str("version", "version_label", true, 9, rep), EffectiveFrom: vm.date("version", "effective_from", rep),
			EffectiveTo: vm.date("version", "effective_to", rep), ChangeNote: optional(vm.str("version", "change_note", false, 1000, rep)),
			ContentLanguage: vm.str("version", "content_language", false, 2, rep), Row: vn.Line}
	}
	for _, n := range doc.list("", "competencies", rep) {
		cm, ok := asMapping(n, "competencies", []string{"code", "name", "kind", "subject_code"}, rep)
		if !ok {
			continue
		}
		f.Competencies = append(f.Competencies, dto.FileCompetency{Code: cm.str("competencies", "code", true, 64, rep),
			Name: cm.name("competencies", "name", rep), Kind: cm.str("competencies", "kind", true, 64, rep),
			SubjectCode: cm.str("competencies", "subject_code", false, 64, rep), Row: n.Line})
	}
	sgNodes := doc.list("", "subject_grades", rep)
	if len(sgNodes) == 0 {
		rep.add(doc.node.Line, "subject_grades", ErrRequired, "needs at least one subject × grade")
	}
	for _, n := range sgNodes {
		sm, ok := asMapping(n, "subject_grades", []string{"subject_code", "grade_level", "periods_per_year", "topics"}, rep)
		if !ok {
			continue
		}
		sg := dto.FileSubjectGrade{SubjectCode: sm.str("subject_grades", "subject_code", true, 64, rep),
			GradeLevel: sm.integer("subject_grades", "grade_level", true, rep), Row: n.Line}
		if _, ok := sm.fields["periods_per_year"]; ok {
			p := sm.integer("subject_grades", "periods_per_year", false, rep)
			sg.PeriodsPerYear = &p
		}
		for _, tn := range sm.list("subject_grades", "topics", rep) {
			tm, ok := asMapping(tn, "topics", []string{"code", "name", "parent_code", "sort_order", "objectives"}, rep)
			if !ok {
				continue
			}
			t := dto.FileTopic{Code: tm.str("topics", "code", true, 64, rep), Name: tm.str("topics", "name", true, 255, rep),
				ParentCode: tm.str("topics", "parent_code", false, 64, rep), SortOrder: tm.integer("topics", "sort_order", false, rep), Row: tn.Line}
			for _, on := range tm.list("topics", "objectives", rep) {
				om, ok := asMapping(on, "objectives", []string{"code", "description", "bloom_level", "sort_order", "yccd_ref", "competency_codes",
					"prerequisite_codes"}, rep)
				if !ok {
					continue
				}
				t.Objectives = append(t.Objectives, dto.FileObjective{Code: om.str("objectives", "code", true, 64, rep),
					Description: om.str("objectives", "description", true, 1000, rep), BloomLevel: om.str("objectives", "bloom_level", false, 64, rep),
					SortOrder: om.integer("objectives", "sort_order", false, rep), YccdRef: om.str("objectives", "yccd_ref", false, 64, rep),
					CompetencyCodes: om.strings("objectives", "competency_codes", rep), PrerequisiteCodes: om.strings("objectives", "prerequisite_codes", rep),
					Row: on.Line})
			}
			sg.Topics = append(sg.Topics, t)
		}
		f.SubjectGrades = append(f.SubjectGrades, sg)
	}
	return f, rep.errs
}

// ---------- CSV

// CSVColumns are the columns of a curriculum CSV file, in order.
var CSVColumns = []string{"curriculum_code", "curriculum_name_vi", "version_label", "subject_code", "grade_level", "topic_code", "topic_name",
	"parent_topic_code", "objective_code", "description", "bloom_level", "yccd_ref", "prerequisite_codes"}

func parseCSV(r io.Reader) (dto.CurriculumFile, []dto.ImportRowError) {
	rep := &report{}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = len(CSVColumns)
	head, err := cr.Read()
	if err != nil {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: ErrInvalidFormat, Message: "the file is not valid CSV"}}
	}
	if len(head) > 0 {
		head[0] = strings.TrimPrefix(head[0], "\uFEFF")
	}
	if !slices.Equal(head, CSVColumns) {
		return dto.CurriculumFile{}, []dto.ImportRowError{{Row: 1, Code: ErrInvalidFormat, Message: "the header must be " + strings.Join(CSVColumns, ",")}}
	}
	f := dto.CurriculumFile{FormatVersion: 1}
	type sgKey struct {
		subject string
		grade   int
	}
	sgIndex := map[sgKey]int{}
	topicIndex := map[string][2]int{}
	line := 1
	for {
		rec, err := cr.Read()
		line++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			rep.add(line, "", ErrInvalidFormat, "the row is not valid CSV")
			return f, rep.errs
		}
		col := func(name string) string { return strings.TrimSpace(rec[slices.Index(CSVColumns, name)]) }
		if f.Curriculum.Code == "" {
			f.Curriculum = dto.FileCurriculum{Code: col("curriculum_code"), Name: map[string]string{"vi": col("curriculum_name_vi")}, Row: line}
			f.Version = dto.FileVersion{Label: col("version_label"), ContentLanguage: "vi", Row: line}
		} else if col("curriculum_code") != f.Curriculum.Code || col("version_label") != f.Version.Label {
			rep.add(line, "curriculum_code", ErrInvalid, "every row must have the same curriculum_code and version_label")
			continue
		}
		grade, err := strconv.Atoi(col("grade_level"))
		if err != nil {
			rep.add(line, "grade_level", ErrInvalid, "must be an integer")
			continue
		}
		k := sgKey{col("subject_code"), grade}
		si, ok := sgIndex[k]
		if !ok {
			si = len(f.SubjectGrades)
			sgIndex[k] = si
			f.SubjectGrades = append(f.SubjectGrades, dto.FileSubjectGrade{SubjectCode: k.subject, GradeLevel: grade, Row: line})
		}
		tcode := col("topic_code")
		if tcode == "" {
			rep.add(line, "topic_code", ErrRequired, "is required")
			continue
		}
		pos, ok := topicIndex[tcode]
		if !ok {
			pos = [2]int{si, len(f.SubjectGrades[si].Topics)}
			topicIndex[tcode] = pos
			f.SubjectGrades[si].Topics = append(f.SubjectGrades[si].Topics, dto.FileTopic{Code: tcode, Name: col("topic_name"),
				ParentCode: col("parent_topic_code"), Row: line})
		} else if pos[0] != si {
			rep.add(line, "topic_code", ErrInvalid, "the topic belongs to another subject × grade")
			continue
		}
		if col("objective_code") == "" {
			continue
		}
		var pre []string
		if p := col("prerequisite_codes"); p != "" {
			pre = strings.Split(p, "|")
		}
		t := &f.SubjectGrades[pos[0]].Topics[pos[1]]
		t.Objectives = append(t.Objectives, dto.FileObjective{Code: col("objective_code"), Description: col("description"),
			BloomLevel: col("bloom_level"), YccdRef: col("yccd_ref"), PrerequisiteCodes: pre, Row: line})
	}
	if f.Curriculum.Code == "" {
		rep.add(2, "", ErrRequired, "the file has no row")
	}
	return f, rep.errs
}

// ---------- export

type yamlName map[string]string

type yamlFile struct {
	FormatVersion int               `yaml:"format_version"`
	Curriculum    yamlCurriculum    `yaml:"curriculum"`
	Version       yamlVersion       `yaml:"version"`
	Competencies  []yamlCompetency  `yaml:"competencies,omitempty"`
	SubjectGrades []yamlSubjectGrad `yaml:"subject_grades"`
}

type yamlCurriculum struct {
	Code      string   `yaml:"code"`
	Name      yamlName `yaml:"name"`
	Framework string   `yaml:"framework,omitempty"`
}

type yamlVersion struct {
	Label           string `yaml:"version_label"`
	EffectiveFrom   string `yaml:"effective_from,omitempty"`
	EffectiveTo     string `yaml:"effective_to,omitempty"`
	ChangeNote      string `yaml:"change_note,omitempty"`
	ContentLanguage string `yaml:"content_language,omitempty"`
}

type yamlCompetency struct {
	Code        string   `yaml:"code"`
	Name        yamlName `yaml:"name"`
	Kind        string   `yaml:"kind"`
	SubjectCode string   `yaml:"subject_code,omitempty"`
}

type yamlSubjectGrad struct {
	SubjectCode    string      `yaml:"subject_code"`
	GradeLevel     int         `yaml:"grade_level"`
	PeriodsPerYear *int        `yaml:"periods_per_year,omitempty"`
	Topics         []yamlTopic `yaml:"topics"`
}

type yamlTopic struct {
	Code       string          `yaml:"code"`
	Name       string          `yaml:"name"`
	ParentCode string          `yaml:"parent_code,omitempty"`
	SortOrder  int             `yaml:"sort_order,omitempty"`
	Objectives []yamlObjective `yaml:"objectives,omitempty"`
}

type yamlObjective struct {
	Code              string   `yaml:"code"`
	Description       string   `yaml:"description"`
	BloomLevel        string   `yaml:"bloom_level,omitempty"`
	SortOrder         int      `yaml:"sort_order,omitempty"`
	YccdRef           string   `yaml:"yccd_ref,omitempty"`
	CompetencyCodes   []string `yaml:"competency_codes,omitempty"`
	PrerequisiteCodes []string `yaml:"prerequisite_codes,omitempty"`
}

func dateOf(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}

// Export implements port.CurriculumCodec.
func (Codec) Export(format string, f dto.CurriculumFile) ([]byte, error) {
	switch format {
	case port.FormatYAML:
		y := yamlFile{FormatVersion: 1, Curriculum: yamlCurriculum{Code: f.Curriculum.Code, Name: f.Curriculum.Name},
			Version: yamlVersion{Label: f.Version.Label, EffectiveFrom: dateOf(f.Version.EffectiveFrom), EffectiveTo: dateOf(f.Version.EffectiveTo),
				ContentLanguage: f.Version.ContentLanguage}}
		if f.Curriculum.Framework != nil {
			y.Curriculum.Framework = *f.Curriculum.Framework
		}
		if f.Version.ChangeNote != nil {
			y.Version.ChangeNote = *f.Version.ChangeNote
		}
		for _, c := range f.Competencies {
			y.Competencies = append(y.Competencies, yamlCompetency{Code: c.Code, Name: c.Name, Kind: c.Kind, SubjectCode: c.SubjectCode})
		}
		for _, sg := range f.SubjectGrades {
			ys := yamlSubjectGrad{SubjectCode: sg.SubjectCode, GradeLevel: sg.GradeLevel, PeriodsPerYear: sg.PeriodsPerYear, Topics: []yamlTopic{}}
			for _, t := range sg.Topics {
				yt := yamlTopic{Code: t.Code, Name: t.Name, ParentCode: t.ParentCode, SortOrder: t.SortOrder}
				for _, o := range t.Objectives {
					yt.Objectives = append(yt.Objectives, yamlObjective{Code: o.Code, Description: o.Description, BloomLevel: o.BloomLevel,
						SortOrder: o.SortOrder, YccdRef: o.YccdRef, CompetencyCodes: o.CompetencyCodes, PrerequisiteCodes: o.PrerequisiteCodes})
				}
				ys.Topics = append(ys.Topics, yt)
			}
			y.SubjectGrades = append(y.SubjectGrades, ys)
		}
		var b bytes.Buffer
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		if err := enc.Encode(y); err != nil {
			return nil, err
		}
		return b.Bytes(), enc.Close()
	case port.FormatCSV:
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		if err := w.Write(CSVColumns); err != nil {
			return nil, err
		}
		for _, sg := range f.SubjectGrades {
			for _, t := range sg.Topics {
				base := []string{f.Curriculum.Code, f.Curriculum.Name["vi"], f.Version.Label, sg.SubjectCode, strconv.Itoa(sg.GradeLevel), t.Code, t.Name,
					t.ParentCode}
				if len(t.Objectives) == 0 {
					if err := w.Write(append(base, "", "", "", "", "")); err != nil {
						return nil, err
					}
				}
				for _, o := range t.Objectives {
					row := append(slices.Clone(base), o.Code, o.Description, o.BloomLevel, o.YccdRef, strings.Join(o.PrerequisiteCodes, "|"))
					if err := w.Write(row); err != nil {
						return nil, err
					}
				}
			}
		}
		w.Flush()
		return b.Bytes(), w.Error()
	}
	return nil, fmt.Errorf("importer: unknown format %q", format)
}
