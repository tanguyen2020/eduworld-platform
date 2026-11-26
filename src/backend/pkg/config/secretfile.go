package config

import (
	"fmt"
	"os"
	"strings"
)

// ReadSecretFile reads the Docker secret file named by the variable varName (X_FILE, F130-AC-11). A missing,
// unreadable or empty file is an error that names the variable only — never the path content or a value — so
// the binary stops at start-up with a log that is safe to keep (AGENTS R4, R14).
func ReadSecretFile(varName, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: cannot read secret file", varName)
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("%s: secret file is empty", varName)
	}
	return v, nil
}
