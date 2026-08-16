//go:build linux

package opener

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func openURL(urlStr string) error {
	return startDetached("xdg-open", urlStr)
}

// openSMBShare monta o share via GVFS guest e abre um symlink estável em
// ~/XVPN/… . O COSMIC Files (Pop!_OS) frequentemente sobe o processo com o
// path cru do FUSE (`smb-share:server=…,share=…`) sem mostrar janela — os
// `:`/`,` no nome quebram o foco no compositor. O symlink sem caracteres
// especiais + FileManager1/gio open corrige o clique em Compartilhado /
// Meus arquivos.
func openSMBShare(host, share string) error {
	if p := localSharePath(share); isCIFSMount(p) {
		return openLocalDir(p)
	}
	if err := ensureSMBMounted(host, share); err != nil {
		return err
	}
	path, err := ensureUserShareLink(host, share)
	if err != nil {
		// Último recurso: path FUSE cru (melhor que falhar em silêncio).
		if p := resolveGVFSMount(host, share); p != "" {
			return openLocalDir(p)
		}
		return err
	}
	return openLocalDir(path)
}

func ensureSMBMounted(host, share string) error {
	if isCIFSMount(localSharePath(share)) {
		return nil
	}
	if resolveGVFSMount(host, share) == "" {
		uri := fmt.Sprintf("smb://%s/%s", host, share)
		out, err := exec.Command("gio", "mount", "--anonymous", uri).CombinedOutput()
		msg := strings.TrimSpace(string(out))
		ok := err == nil || strings.Contains(msg, "already mounted")
		if !ok && resolveGVFSMount(host, share) == "" {
			if msg == "" && err != nil {
				msg = err.Error()
			}
			return fmt.Errorf("montando %s: %s", uri, msg)
		}
		if waitGVFSMount(host, share, 3*time.Second) == "" {
			return fmt.Errorf("montando %s: caminho local GVFS não apareceu a tempo", uri)
		}
	}
	// Atualiza ~/XVPN even when already mounted (Connect / bandeja).
	_, _ = ensureUserShareLink(host, share)
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

func resolveGVFSMount(host, share string) string {
	return pickGVFSEntry(gvfsRoot(), host, share)
}

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

// shareLinkName é o nome amigável em ~/XVPN (sem ':'/',' do FUSE GVFS).
func localSharePath(share string) string {
	dir, err := xvpnLinksDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, shareLinkName(share))
}

func unescapeProcMount(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			n, err := strconv.ParseUint(s[i+1:i+4], 8, 8)
			if err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isCIFSMount(mountpoint string) bool {
	if mountpoint == "" {
		return false
	}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()
	want := strings.TrimRight(mountpoint, "/")
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		got := strings.TrimRight(unescapeProcMount(fields[1]), "/")
		if got == want && (fields[2] == "cifs" || fields[2] == "smb3") {
			return true
		}
	}
	return false
}

func shareLinkName(share string) string {
	if share == "shared" {
		return "Compartilhado"
	}
	if strings.HasPrefix(share, "home-") {
		return "Meus arquivos"
	}
	return share
}

func xvpnLinksDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "XVPN")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureUserShareLink aponta ~/XVPN/<nome> → mount GVFS atual.
func ensureUserShareLink(host, share string) (string, error) {
	gvfs := resolveGVFSMount(host, share)
	if gvfs == "" {
		return "", fmt.Errorf("mount //%s/%s ausente", host, share)
	}
	dir, err := xvpnLinksDir()
	if err != nil {
		return "", err
	}
	link := filepath.Join(dir, shareLinkName(share))
	if target, err := os.Readlink(link); err == nil && target == gvfs {
		return link, nil
	}
	_ = os.Remove(link)
	if err := os.Symlink(gvfs, link); err != nil {
		return "", fmt.Errorf("criando atalho %s: %w", link, err)
	}
	return link, nil
}

// unmountServerSMBShares tira do SO tudo que o Connect/opener criou para
// este servidor: mounts GVFS (guest e legados com user=), symlinks
// ~/XVPN e pasta-órfã no Desktop no formato smb-share:server=….
func unmountServerSMBShares(host string) error {
	var errs []string

	// Desmonta pelo path FUSE e pela URI — cobre guest e user=legado.
	root := gvfsRoot()
	if entries, err := os.ReadDir(root); err == nil {
		prefix := "smb-share:server=" + host
		for _, e := range entries {
			name := e.Name()
			if name != prefix && !strings.HasPrefix(name, prefix+",") {
				continue
			}
			path := filepath.Join(root, name)
			if out, err := exec.Command("gio", "mount", "-u", "-f", path).CombinedOutput(); err != nil {
				msg := strings.TrimSpace(string(out))
				if msg == "" {
					msg = err.Error()
				}
				// Já desmontado / inexistente — ok.
				if !strings.Contains(strings.ToLower(msg), "not mounted") &&
					!strings.Contains(strings.ToLower(msg), "não está montado") {
					errs = append(errs, msg)
				}
			}
		}
	}

	// URIs explícitas (shared + o que os symlinks apontavam).
	for _, uri := range smbURIsForHost(host) {
		_ = exec.Command("gio", "mount", "-u", "-f", uri).Run()
	}

	removeUserShareLinks()
	removeDesktopSMBRemnants(host)

	// GVFS às vezes demora a retirar a entrada FUSE após -u.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !hostHasGVFSMounts(host) {
			break
		}
		time.Sleep(50 * time.Millisecond)
		for _, uri := range smbURIsForHost(host) {
			_ = exec.Command("gio", "mount", "-u", "-f", uri).Run()
		}
		if entries, err := os.ReadDir(gvfsRoot()); err == nil {
			prefix := "smb-share:server=" + host
			for _, e := range entries {
				if e.Name() == prefix || strings.HasPrefix(e.Name(), prefix+",") {
					_ = exec.Command("gio", "mount", "-u", "-f", filepath.Join(gvfsRoot(), e.Name())).Run()
				}
			}
		}
	}

	if hostHasGVFSMounts(host) {
		errs = append(errs, "ainda há mounts GVFS após desmontar")
	}

	if len(errs) > 0 {
		return fmt.Errorf("desmontando SMB de %s: %s", host, strings.Join(errs, "; "))
	}
	return nil
}

func hostHasGVFSMounts(host string) bool {
	entries, err := os.ReadDir(gvfsRoot())
	if err != nil {
		return false
	}
	prefix := "smb-share:server=" + host
	for _, e := range entries {
		if e.Name() == prefix || strings.HasPrefix(e.Name(), prefix+",") {
			return true
		}
	}
	return false
}

func smbURIsForHost(host string) []string {
	uris := []string{
		fmt.Sprintf("smb://%s/shared", host),
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return uris
	}
	dir := filepath.Join(home, "XVPN")
	for _, name := range []string{"Compartilhado", "Meus arquivos"} {
		link := filepath.Join(dir, name)
		if target, err := os.Readlink(link); err == nil {
			base := filepath.Base(target)
			const mark = ",share="
			if i := strings.Index(base, mark); i >= 0 {
				share := base[i+len(mark):]
				if j := strings.Index(share, ","); j >= 0 {
					share = share[:j]
				}
				uris = append(uris, fmt.Sprintf("smb://%s/%s", host, share))
			}
		}
	}
	return uris
}

func removeUserShareLinks() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, "XVPN")
	for _, name := range []string{"Compartilhado", "Meus arquivos"} {
		_ = os.Remove(filepath.Join(dir, name))
	}
	// Remove ~/XVPN se ficou vazio.
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

// removeDesktopSMBRemnants apaga pastas/ícones no Desktop criados a partir
// de mounts SMB deste host (legado user=xvpntest e ícones "share on host").
func removeDesktopSMBRemnants(host string) {
	desktop := desktopDir()
	if desktop == "" {
		return
	}
	entries, err := os.ReadDir(desktop)
	if err != nil {
		return
	}
	prefix := "smb-share:server=" + host
	suffix := " on " + host
	for _, e := range entries {
		name := e.Name()
		match := strings.HasPrefix(name, prefix) ||
			strings.HasSuffix(name, suffix) ||
			name == host
		if !match {
			continue
		}
		path := filepath.Join(desktop, name)
		// Só remove dir/symlink — nunca .desktop de apps.
		fi, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(path)
			continue
		}
		if fi.IsDir() {
			_ = os.RemoveAll(path)
		}
	}
}

func desktopDir() string {
	if out, err := exec.Command("xdg-user-dir", "DESKTOP").Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			return d
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Desktop")
}

func openLocalDir(path string) error {
	// 1) D-Bus FileManager1 — pede ao gerenciador já rodando para focar
	// a pasta (no Cosmic, lançar outro cosmic-files muitas vezes não
	// levanta janela visível).
	if err := showFoldersDBus(path); err == nil {
		return nil
	}
	// 2) gio open no symlink (ativa o handler inode/directory).
	if err := startDetached("gio", "open", path); err == nil {
		return nil
	}
	// 3) Executável do gerenciador padrão.
	if cmd, args := defaultFileManagerCommand(path); cmd != "" {
		if err := startDetached(cmd, args...); err == nil {
			return nil
		}
	}
	return startDetached("xdg-open", path)
}

func showFoldersDBus(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	uri := (&url.URL{Scheme: "file", Path: abs}).String()
	out, err := exec.Command(
		"gdbus", "call", "--session",
		"--dest", "org.freedesktop.FileManager1",
		"--object-path", "/org/freedesktop/FileManager1",
		"--method", "org.freedesktop.FileManager1.ShowFolders",
		fmt.Sprintf("['%s']", uri),
		`""`,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("FileManager1.ShowFolders: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

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
