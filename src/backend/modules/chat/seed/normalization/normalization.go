// Package normalization embeds the versioned normalisation and PII configuration of the moderation pipeline (F116 T003,
// MODERATION §3.1, §3.3). It holds no sensitive word (R15).
package normalization

import _ "embed"

// V1 is normalization.v1.json.
//
//go:embed normalization.v1.json
var V1 []byte
