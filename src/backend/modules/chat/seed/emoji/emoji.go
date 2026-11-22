// Package emoji embeds the platform set of allowed emoji and reaction codes (CHAT_SYSTEM §7.5–§7.6, F118 T002).
package emoji

import _ "embed"

// JSON is emoji_sets.json.
//
//go:embed emoji_sets.json
var JSON []byte
