// Package regression embeds the placeholder regression corpus of the sensitive-term catalogue (F116 T022, MODERATION
// §11.4). It holds no sensitive word (R15): the positive catalogue samples are generated from the placeholder terms of
// the candidate catalogue; the operational corpus (≥ 2,000 labelled sentences) lives outside the repository.
package regression

import _ "embed"

// V1 is corpus.v1.json.
//
//go:embed corpus.v1.json
var V1 []byte
