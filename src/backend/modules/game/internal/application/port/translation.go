package port

import (
	"errors"
	"io"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// ErrMalformedFile: the translation file is not a valid XLIFF 2.1 / CSV file (the import job fails, F082-AC-14).
var ErrMalformedFile = errors.New("port: malformed translation file")

// TranslationCodec reads and writes one translation file format (F082, research R4): XLIFF 2.1 or CSV.
type TranslationCodec interface {
	// Extension and ContentType of the written file.
	Extension() string
	ContentType() string
	Encode(w io.Writer, f i18ntext.File) error
	// Decode returns ErrMalformedFile (wrapped) for a file that cannot be read.
	Decode(r io.Reader) (i18ntext.File, error)
}
