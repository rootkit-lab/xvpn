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
