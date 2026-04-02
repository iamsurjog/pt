package scripts

import (
	"os"
	"strings"
)

func GetVenvPath() string {
	return os.Getenv("VIRTUAL_ENV")
}

// parsePackageName extracts the package name from a requirement string.
// It ignores version specifiers, extras, and environment markers.
func parsePackageName(req string) string {
	// Remove environment markers (after ';')
	if idx := strings.Index(req, ";"); idx != -1 {
		req = req[:idx]
	}
	// Remove extras (e.g., "package[extra]")
	if idx := strings.Index(req, "["); idx != -1 {
		req = req[:idx]
	}
	// Trim spaces
	req = strings.TrimSpace(req)
	// Split by version specifier characters
	for i, ch := range req {
		if ch == '<' || ch == '>' || ch == '=' || ch == '!' || ch == '~' {
			return strings.TrimSpace(req[:i])
		}
	}
	return req
}
