package agent

import (
	"path/filepath"
	"os"
)

// Checks for a file
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// Creates payload from []bytes
func writePayloadBytes(location string, payload []byte) {
	dst, _ := os.Create(location)
	defer dst.Close()
	_, _ = dst.Write(payload)
	os.Chmod(location, 0700)
}