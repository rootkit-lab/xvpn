//go:build windows

package opener

import (
	"fmt"
	"os/exec"
)

// "start" é builtin do cmd.exe, não um executável — precisa passar por
// "cmd /c". O argumento vazio depois de "start" é o título da janela
// (obrigatório quando o alvo pode conter espaços/aspas).
func openURL(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}

func openSMBShare(host, share string) error {
	return exec.Command("cmd", "/c", "start", "", fmt.Sprintf(`\\%s\%s`, host, share)).Start()
}

// ensureSMBMounted no Windows: Explorer resolve o UNC no open; nada a
// pré-montar no espaço do usuário sem net use interativo.
func ensureSMBMounted(host, share string) error {
	return nil
}

func unmountServerSMBShares(host string) error {
	// Remove mapeamentos UNC deste host (se o Explorer tiver feito net use).
	_ = exec.Command("cmd", "/c", "net", "use", fmt.Sprintf(`\\%s\*`, host), "/delete", "/y").Run()
	_ = exec.Command("cmd", "/c", "net", "use", fmt.Sprintf(`\\%s\shared`, host), "/delete", "/y").Run()
	return nil
}
