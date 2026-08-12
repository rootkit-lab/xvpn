//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// defaultStatePath: %ProgramData% é acessível ao LocalSystem (identidade em
// que o Windows Service do helper roda) e persiste entre reinicializações.
func defaultStatePath() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	return filepath.Join(programData, "XVPN", "device.json")
}
