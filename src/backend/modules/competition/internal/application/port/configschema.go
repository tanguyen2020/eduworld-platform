package port

import "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"

// ConfigSchemas validates a competition config payload against the JSON Schema of its type (specs/schemas, F108-AC-04):
// the violations name the failing fields ("payload/…"); none when the payload conforms.
type ConfigSchemas interface {
	Validate(configType string, raw []byte) ([]gameconfig.Violation, error)
}
