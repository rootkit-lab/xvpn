//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// desktopFileName segue a mesma convenção de nome do atalho instalado
// manualmente (ver client/build/linux/xvpn-client.desktop) — mas este
// arquivo mora em ~/.config/autostart, não em /usr/share/applications, e
// é gerenciado inteiramente por este pacote (criado/removido pela GUI).
const desktopFileName = "xvpn-client-autostart.desktop"

func autostartFilePath() string {
	return filepath.Join(xdg.ConfigHome, "autostart", desktopFileName)
}

// removeLegacyAutostartCopy apaga ~/.config/autostart/xvpn-client.desktop,
// cópia do atalho de menu que o README antigo pedia para autostart e que
// competia com xvpn-client-autostart.desktop (duas GUIs no login).
func removeLegacyAutostartCopy() {
	legacy := filepath.Join(xdg.ConfigHome, "autostart", "xvpn-client.desktop")
	_ = os.Remove(legacy)
}

func isEnabled() (bool, error) {
	_, err := os.Stat(autostartFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("verificando autostart: %w", err)
	}
	return true, nil
}

func setEnabled(enabled bool) error {
	path := autostartFilePath()
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removendo entrada de autostart: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("descobrindo caminho do executável: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criando diretório de autostart: %w", err)
	}

	// X-GNOME-Autostart-enabled=true é reconhecido pela maioria dos
	// ambientes (GNOME, KDE, XFCE, COSMIC) além do GNOME propriamente —
	// virou de fato o padrão informal do freedesktop.org para isso.
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=XVPN
Comment=Inicia o cliente XVPN minimizado na bandeja
Exec=%s
Icon=xvpn-client
Terminal=false
X-GNOME-Autostart-enabled=true
`, exe)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("gravando entrada de autostart: %w", err)
	}
	return nil
}
