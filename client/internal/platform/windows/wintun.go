//go:build windows

package windows

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// wintunFS embute o diretório wintun/ do repositório. wintun.dll (o driver
// TUN de alta performance da WireGuard LLC, https://www.wintun.net/) NÃO é
// commitado — é um binário de terceiros, baixado sob demanda por
// build/windows/fetch-wintun.ps1 antes de compilar para Windows (mesma
// lógica do placeholder em server/internal/webui/dist, ver PLAN.md §11.1).
// Só wintun/placeholder.txt é rastreado, garantindo que o `go:embed` nunca
// falhe num checkout limpo mesmo antes do fetch.
//
//go:embed wintun
var wintunFS embed.FS

// ensureWintunDLL extrai o wintun.dll embutido para o mesmo diretório do
// executável em execução. golang.zx2c4.com/wireguard/tun carrega
// "wintun.dll" via LoadLibrary, que segue a ordem de busca padrão do
// Windows — o diretório do processo em execução vem antes de
// C:\Windows\System32, então colocá-lo ali é suficiente, sem precisar
// alterar PATH ou instalar nada em System32.
func ensureWintunDLL() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("localizando executável em execução: %w", err)
	}
	dest := filepath.Join(filepath.Dir(exePath), "wintun.dll")
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	data, err := wintunFS.ReadFile("wintun/wintun.dll")
	if err != nil {
		return fmt.Errorf("wintun.dll não embutido no binário — rode client/build/windows/fetch-wintun.ps1 antes de compilar para Windows: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return fmt.Errorf("gravando wintun.dll em %q: %w", dest, err)
	}
	return nil
}
