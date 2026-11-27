package port

import (
	"context"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
)

// DocxConverter converts a Word .docx into the neutral paragraph model of the import lessons_docx (F028): the zip is
// checked first (size, entries, macros), then converted by Pandoc as an external process (infrastructure/docximport,
// research R1). Errors are the file-level failures below (or another error for an unexpected failure).
type DocxConverter interface {
	Convert(ctx context.Context, data []byte) (docximport.Document, error)
	// Media returns an embedded file of the .docx (an image of the document, path as in Inline.Path).
	Media(data []byte, path string) ([]byte, error)
}

// File-level failures of a conversion; Code is the ContentImportJob.errors[].code.
var (
	ErrDocxInvalid       = DocxError{Code: docximport.FileInvalidFormat, Message: "the file is not a valid Word .docx document"}
	ErrDocxMacro         = DocxError{Code: docximport.FileMacroNotAllowed, Message: "the file contains macros"}
	ErrDocxTooComplex    = DocxError{Code: docximport.FileTooComplex, Message: "the file has too many parts or is too large once unpacked"}
	ErrDocxTimeout       = DocxError{Code: docximport.FileConversionTimeout, Message: "the conversion took longer than docx_convert_timeout"}
	ErrDocxNoConverter   = DocxError{Code: docximport.FileConverterUnavailable, Message: "the Word converter is not available on this worker"}
	ErrDocxMediaNotFound = errors.New("port: embedded file not found")
)

// DocxError is a file-level failure of a conversion.
type DocxError struct {
	Code    string
	Message string
}

func (e DocxError) Error() string { return "docx: " + e.Code }
