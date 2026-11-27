// Package docximport is the domain of the Word import lessons_docx (F028, CONTENT_PRODUCTION_PLAN §8): a neutral
// model of the paragraphs of a converted .docx (text runs, Office Math formulas as LaTeX, images, MathType OLE
// objects), the template that splits it into questions ("Câu n.", options "A."…, the bold or "Đáp án:" answer,
// "Gợi ý k:", "Giải thích:", "LO:", "Độ khó:") and the authoring content of single/multiple choice questions built
// from them, with the issues of every question. Formulas go through the shared allowlist of mathx (ADR-0055 §7).
// Pure Go, no I/O: the conversion (Pandoc) and the zip checks belong to infrastructure/docximport.
package docximport

// InlineKind is the kind of a piece of a paragraph.
type InlineKind uint8

// Inline kinds.
const (
	Text  InlineKind = iota + 1 // text run (Bold = in bold)
	Math                        // Office Math formula converted to LaTeX (Display = own line)
	Image                       // embedded picture (Path in the zip, Alt text)
	OLE                         // MathType/Equation OLE object (no Office Math formula)
	Break                       // line break inside a paragraph
)

// Inline is one piece of a paragraph.
type Inline struct {
	Kind    InlineKind
	Text    string
	Bold    bool
	Latex   string
	Display bool
	Path    string // Image: part of the zip (word/media/…)
	Alt     string // Image: alternative text
}

// Block is one paragraph of the document; Notes are the issues the conversion found in it (a removed link, a
// table…), reported on the question holding it.
type Block struct {
	Inlines []Inline
	Notes   []Issue
}

// Document is a converted .docx: its paragraphs in order.
type Document struct {
	Blocks []Block
}

// Severity of an issue: an error makes a question not importable, a warning does not.
type Severity string

// Severities.
const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
)

// Issue codes of a question (ContentImportItemIssue.code, CONTENT_PRODUCTION_PLAN §8).
const (
	CodeStemEmpty            = "STEM_EMPTY"
	CodeOptionsTooFew        = "OPTIONS_TOO_FEW"
	CodeOptionsTooMany       = "OPTIONS_TOO_MANY"
	CodeOptionOrder          = "OPTION_ORDER"
	CodeOptionEmpty          = "OPTION_EMPTY"
	CodeAnswerMissing        = "ANSWER_MISSING"
	CodeAnswerConflict       = "ANSWER_CONFLICT"
	CodeAnswerUnknownOption  = "ANSWER_UNKNOWN_OPTION"
	CodeMetadataMissing      = "METADATA_MISSING"
	CodeDifficultyInvalid    = "DIFFICULTY_INVALID"
	CodeUnknownCode          = "UNKNOWN_CODE"
	CodeMathNotAllowed       = "MATH_NOT_ALLOWED"
	CodeFormulaNeedsRewrite  = "FORMULA_NEEDS_REWRITE"
	CodeFormulasTooMany      = "FORMULAS_TOO_MANY"
	CodeImageAltMissing      = "IMAGE_ALT_MISSING"
	CodeImageTypeNotAllowed  = "IMAGE_TYPE_NOT_ALLOWED"
	CodeImageNotFound        = "IMAGE_NOT_FOUND"
	CodeImagesTooMany        = "IMAGES_TOO_MANY"
	CodeUnsupportedContent   = "UNSUPPORTED_CONTENT"
	CodeUnsupportedFormat    = "UNSUPPORTED_QUESTION_FORMAT"
	CodeTextTooLong          = "TEXT_TOO_LONG"
	CodeHintsTooMany         = "HINTS_TOO_MANY"
	CodeExternalLinkRemoved  = "EXTERNAL_LINK_REMOVED"
	CodeFootnoteRemoved      = "FOOTNOTE_REMOVED"
	CodeListFlattened        = "LIST_FLATTENED"
	CodeTextScriptConverted  = "TEXT_SCRIPT_CONVERTED"
	CodeQuestionNumberRepeat = "QUESTION_NUMBER_REPEATED"
)

// File-level codes of a lessons_docx job (ContentImportJob.errors[].code with field "file").
const (
	FileInvalidFormat        = "INVALID_FORMAT"
	FileMacroNotAllowed      = "MACRO_NOT_ALLOWED"
	FileTooComplex           = "DOCX_TOO_COMPLEX"
	FileConversionTimeout    = "CONVERSION_TIMEOUT"
	FileConverterUnavailable = "CONVERTER_UNAVAILABLE"
	FileNoQuestions          = "NO_QUESTIONS"
	FileTooManyQuestions     = "TOO_MANY_QUESTIONS"
)

// Issue is one finding on a question: a code of the catalog, the field it concerns and a message safe to show to
// the author (it never holds a formula or the content of the question).
type Issue struct {
	Code     string   `json:"code"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message,omitempty"`
	Severity Severity `json:"severity"`
	// Source is "validator" for a refusal of the validators of createQuestion when the draft was created (F028-AC-12).
	Source string `json:"source,omitempty"`
}

// SourceValidator marks the issues of a refused draft creation.
const SourceValidator = "validator"

// Err is an error issue.
func Err(code, field, msg string) Issue {
	return Issue{Code: code, Field: field, Message: msg, Severity: SevError}
}

// Warn is a warning issue.
func Warn(code, field, msg string) Issue {
	return Issue{Code: code, Field: field, Message: msg, Severity: SevWarning}
}

// Statuses of an import item (data-model §2).
const (
	StatusReady       = "ready"
	StatusWarning     = "warning"
	StatusError       = "error"
	StatusCreated     = "created"
	StatusDraftFailed = "draft_failed"
)

// StatusOf is the analysis status of a question with these issues.
func StatusOf(issues []Issue) string {
	st := StatusReady
	for _, i := range issues {
		if i.Severity == SevError {
			return StatusError
		}
		st = StatusWarning
	}
	return st
}

// Importable reports whether a draft can be created from an item in status st.
func Importable(st string) bool {
	return st == StatusReady || st == StatusWarning || st == StatusDraftFailed
}

// MaxIssues is the number of issues kept per question (ContentImportItem.issues maxItems).
const MaxIssues = 50

// Limits of one question (question.schema.json and CONFIG_DEFAULTS).
type Limits struct {
	MaxHints    int // hints maxItems 5
	MaxFormulas int // math_assets maxItems 50
	MaxImages   int // images of one question
	TextMax     int // i18nText maxLength 1000
	LatexMax    int // richBlock latex maxLength 1000
	PreviewMax  int // stem_preview varchar(200)
	AltMax      int // media_assets.alt_text varchar(500)
}

// DefaultLimits are the limits of the schemas.
var DefaultLimits = Limits{MaxHints: 5, MaxFormulas: 50, MaxImages: 20, TextMax: 1000, LatexMax: 1000, PreviewMax: 200, AltMax: 500}
