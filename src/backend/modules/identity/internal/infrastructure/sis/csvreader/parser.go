package csvreader

import (
	"fmt"
	"io"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
)

// APIKinds maps entity_kinds of createSisImport to the contract DTO (SIS contract 1.3.0: no classes, no enrollments).
var APIKinds = map[string]string{"schools": contract.KindSchool, "students": contract.KindStudent}

// ignoredAPIKinds are the entity kinds removed by SIS contract 1.3.0 (ADR-0095 D6): their file is not read and the run
// ignores it with a warning.
var ignoredAPIKinds = map[string]string{"classes": "class", "enrollments": "enrollment"}

// Parser implements port.SisFileParser with the embedded contract validator.
type Parser struct{ V *contract.Validator }

var _ port.SisFileParser = Parser{}

// Parse reads one file of an API entity kind.
func (p Parser) Parse(kind string, r io.Reader, now time.Time) (port.ParsedFile, error) {
	if k, ok := ignoredAPIKinds[kind]; ok {
		return port.ParsedFile{Kind: k}, nil
	}
	ck, ok := APIKinds[kind]
	if !ok {
		return port.ParsedFile{}, fmt.Errorf("csvreader: unknown kind %q", kind)
	}
	b, err := Read(ck, r, p.V, now)
	if err != nil {
		return port.ParsedFile{}, err
	}
	out := port.ParsedFile{Kind: EntityKind(ck)}
	for _, rec := range b.Records {
		out.Records = append(out.Records, port.ParsedRecord{Row: rec.Row, Data: rec.Data})
	}
	for _, e := range b.Errors {
		out.Violations = append(out.Violations, port.ParsedViolation{Row: e.Row, Column: e.Column, Code: e.Code, Message: e.Message})
	}
	return out, nil
}
