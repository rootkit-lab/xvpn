//go:build windows

package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath é a chave padrão do Windows pra iniciar programas no login
// do usuário atual — não precisa de privilégio de administrador (HKCU,
// não HKLM), consistente com a GUI rodando sem privilégio.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

const valueName = "XVPN"

func isEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("abrindo chave de registro: %w", err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(valueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("lendo valor de autostart: %w", err)
	}
	return true, nil
}

func setEnabled(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("abrindo/criando chave de registro: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("removendo valor de autostart: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("descobrindo caminho do executável: %w", err)
	}
	// Aspas em volta do caminho: o diretório de instalação pode ter
	// espaço (ex.: "C:\Program Files\XVPN\xvpn-client.exe").
	if err := key.SetStringValue(valueName, `"`+exe+`"`); err != nil {
		return fmt.Errorf("gravando valor de autostart: %w", err)
	}
	return nil
}

func removeLegacyAutostartCopy() {}
