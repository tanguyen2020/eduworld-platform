package port

import (
	"io"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// Import kinds and export formats of a curriculum file (ContentImportRequest.kind, exportCurriculumVersion.format).
const (
	KindCurriculumYAML = "curriculum_yaml"
	KindCurriculumCSV  = "curriculum_csv"
	FormatYAML         = "yaml"
	FormatCSV          = "csv"
)

// CurriculumCodec reads and writes curriculum files (YAML, CSV) — specs/schemas/curriculum-import.schema.json.
// Parse reports structural errors per row (unknown field, wrong type, malformed file); the business rules are
// checked by the import use case with the validators of the CMS.
type CurriculumCodec interface {
	Parse(kind string, r io.Reader) (dto.CurriculumFile, []dto.ImportRowError)
	Export(format string, f dto.CurriculumFile) ([]byte, error)
}
