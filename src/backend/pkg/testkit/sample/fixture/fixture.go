// Package fixture embeds the DDL of the sample module used by F003 integration tests.
package fixture

import _ "embed"

// SQL creates schema sample with one table per entx kind.
//
//go:embed sample.sql
var SQL string
