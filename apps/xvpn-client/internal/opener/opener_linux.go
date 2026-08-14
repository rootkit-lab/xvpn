//go:build linux

package opener

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func openURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// openSMBShare abre o compartilhamento SMB no gerenciador de arquivos
// padrão do usuário. Não basta `xdg-open smb://...`: isso só funciona se o
// gerenciador de arquivos tiver se registrado como handler do esquema
// "x-scheme-handler/smb" (via MimeType no próprio .desktop) — nem todo
// gerenciador faz isso. Achado real (ver ROADMAP.md Fase 6): o COSMIC
// Files (padrão no Pop!_OS) só declara "inode/directory", então o
// xdg-open, sem handler pra "smb", caía no navegador padrão em vez do
// gerenciador de arquivos. Resolvemos o gerenciador de arquivos padrão
// pelo mesmo mecanismo que o próprio ambiente usa pra abrir pastas
// (inode/directory) e o executamos diretamente com a URI — funciona com
// qualquer gerenciador que aceite uma URI como argumento (Nautilus,
// Dolphin, Nemo, Thunar, PCManFM, COSMIC Files, etc.) em qualquer
// distro/ambiente, com `gio open` e `xdg-open` como fallback caso a
// resolução do gerenciador padrão falhe por algum motivo.
func openSMBShare(host, share string) error {
	uri := fmt.Sprintf("smb://%s/%s", host, share)

	if cmd, args := defaultFileManagerCommand(uri); cmd != "" {
		if err := exec.Command(cmd, args...).Start(); err == nil {
			return nil
		}
	}
	if err := exec.Command("gio", "open", uri).Start(); err == nil {
		return nil
	}
	return exec.Command("xdg-open", uri).Start()
}

// defaultFileManagerCommand descobre o gerenciador de arquivos padrão do
// usuário (associado ao tipo MIME "inode/directory") e monta o comando
// pronto para rodar com uri como argumento, substituindo os "field codes"
// (%f/%F/%u/%U) da Desktop Entry Specification do freedesktop.org.
func defaultFileManagerCommand(uri string) (string, []string) {
	out, err := exec.Command("xdg-mime", "query", "default", "inode/directory").Output()
	if err != nil {
		return "", nil
	}
	desktopFile := strings.TrimSpace(string(out))
	if desktopFile == "" {
		return "", nil
	}

	path := findDesktopFile(desktopFile)
	if path == "" {
		return "", nil
	}

	execLine := parseExecLine(path)
	if execLine == "" {
		return "", nil
	}

	fields := strings.Fields(execLine)
	if len(fields) == 0 {
		return "", nil
	}

	args := make([]string, 0, len(fields))
	uriConsumed := false
	for _, f := range fields[1:] {
		switch f {
		case "%f", "%F", "%u", "%U":
			args = append(args, uri)
			uriConsumed = true
		case "%i", "%c", "%k":
			// Field codes sem uso aqui (ícone/nome/caminho do próprio
			// .desktop) — fazem sentido só num lançador gráfico, omitidos.
		default:
			args = append(args, f)
		}
	}
	if !uriConsumed {
		args = append(args, uri)
	}
	return fields[0], args
}

// findDesktopFile procura o arquivo .desktop pelo ID nos diretórios padrão
// da Desktop Entry Specification, na ordem de precedência especificada
// (XDG_DATA_HOME antes de cada diretório de XDG_DATA_DIRS).
func findDesktopFile(id string) string {
	var dirs []string
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome != "" {
		dirs = append(dirs, dataHome)
	}

	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	dirs = append(dirs, strings.Split(dataDirs, ":")...)

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "applications", id)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// parseExecLine lê a linha "Exec=" do grupo [Desktop Entry] — não vale a
// pena um parser INI completo só pra isso.
func parseExecLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	inDesktopEntry := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "[Desktop Entry]":
			inDesktopEntry = true
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			inDesktopEntry = false
		case inDesktopEntry && strings.HasPrefix(line, "Exec="):
			return strings.TrimPrefix(line, "Exec=")
		}
	}
	return ""
}
