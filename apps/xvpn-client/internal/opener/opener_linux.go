//go:build linux

package opener

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func openURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// openSMBShare monta o share via GVFS como guest (anônimo) e abre o
// caminho local em /run/user/$UID/gvfs/…. Não passa smb:// direto ao
// gerenciador de arquivos: o COSMIC Files (Pop!_OS) só declara
// MimeType=inode/directory — com URI smb:// ele sobe a janela no Home
// local e ignora o destino (sintoma reportado na validação pós-Fase 14).
//
// --anonymous casa com guest ok = yes no Samba do VPS (Fase 14); sem
// isso o gio pede usuário/senha e o mount interativo falha na GUI.
func openSMBShare(host, share string) error {
	if err := ensureSMBMounted(host, share); err != nil {
		return err
	}
	path := resolveGVFSMount(host, share)
	if path == "" {
		return fmt.Errorf("compartilhamento //%s/%s montado, mas o caminho local GVFS não apareceu", host, share)
	}
	return openLocalDir(path)
}

func ensureSMBMounted(host, share string) error {
	if resolveGVFSMount(host, share) != "" {
		return nil
	}
	uri := fmt.Sprintf("smb://%s/%s", host, share)
	out, err := exec.Command("gio", "mount", "--anonymous", uri).CombinedOutput()
	msg := strings.TrimSpace(string(out))
	ok := err == nil || strings.Contains(msg, "already mounted")
	if !ok && resolveGVFSMount(host, share) == "" {
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("montando %s: %s", uri, msg)
	}
	// gio pode sair 0 antes do FUSE publicar o diretório em
	// /run/user/$UID/gvfs — espera curta evita o botão abrir nada / Home.
	if waitGVFSMount(host, share, 3*time.Second) == "" {
		return fmt.Errorf("montando %s: caminho local GVFS não apareceu a tempo", uri)
	}
	return nil
}

func waitGVFSMount(host, share string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for {
		if p := resolveGVFSMount(host, share); p != "" {
			return p
		}
		if time.Now().After(deadline) {
			return ""
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// resolveGVFSMount acha o diretório FUSE do share. Prefere Stat no path
// anônimo esperado (sem ,user=); ReadDir só para fallback de mounts
// legados com user= (ex.: xvpntest).
func resolveGVFSMount(host, share string) string {
	return pickGVFSEntry(gvfsRoot(), host, share)
}

// pickGVFSEntry escolhe entre mounts anônimo vs user= no diretório GVFS.
// Separado pra testar sem depender de /run/user.
func pickGVFSEntry(root, host, share string) string {
	anonName := fmt.Sprintf("smb-share:server=%s,share=%s", host, share)
	anonPath := filepath.Join(root, anonName)
	if fi, err := os.Stat(anonPath); err == nil && fi.IsDir() {
		return anonPath
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var withUser string
	for _, e := range entries {
		name := e.Name()
		if name == anonName {
			return filepath.Join(root, name)
		}
		if strings.HasPrefix(name, anonName+",user=") {
			withUser = filepath.Join(root, name)
		}
	}
	return withUser
}

func gvfsRoot() string {
	return fmt.Sprintf("/run/user/%d/gvfs", os.Getuid())
}

func openLocalDir(path string) error {
	if cmd, args := defaultFileManagerCommand(path); cmd != "" {
		if err := exec.Command(cmd, args...).Start(); err == nil {
			return nil
		}
	}
	if err := exec.Command("gio", "open", path).Start(); err == nil {
		return nil
	}
	return exec.Command("xdg-open", path).Start()
}

// defaultFileManagerCommand descobre o gerenciador de arquivos padrão do
// usuário (associado ao tipo MIME "inode/directory") e monta o comando
// pronto para rodar com pathOrURI como argumento, substituindo os
// "field codes" (%f/%F/%u/%U) da Desktop Entry Specification.
func defaultFileManagerCommand(pathOrURI string) (string, []string) {
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
			args = append(args, pathOrURI)
			uriConsumed = true
		case "%i", "%c", "%k":
			// Field codes de lançador gráfico — omitidos.
		default:
			args = append(args, f)
		}
	}
	if !uriConsumed {
		args = append(args, pathOrURI)
	}
	return fields[0], args
}

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
