//go:build linux

package helper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (h *Helper) handleMountSMB(raw json.RawMessage) (any, error) {
	var req MountSMBRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parâmetros de mount SMB inválidos: %w", err)
	}
	target, err := resolveSMBMount(req)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Lstat(target.Mountpoint); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(target.Mountpoint); err != nil {
			return nil, fmt.Errorf("removendo atalho GVFS %s: %w", target.Mountpoint, err)
		}
	}
	if err := os.MkdirAll(target.Mountpoint, 0o755); err != nil {
		return nil, fmt.Errorf("criando %s: %w", target.Mountpoint, err)
	}
	if err := os.Chown(target.Mountpoint, target.UID, target.GID); err != nil {
		return nil, fmt.Errorf("chown %s: %w", target.Mountpoint, err)
	}
	if isCIFSMount(target.Mountpoint) {
		return map[string]string{"mountpoint": target.Mountpoint}, nil
	}
	if _, err := exec.LookPath("mount.cifs"); err != nil {
		return nil, fmt.Errorf("cifs-utils não instalado (mount.cifs) — o cliente cai no GVFS")
	}
	src := fmt.Sprintf("//%s/%s", target.Host, target.Share)
	opts := fmt.Sprintf(
		"guest,vers=3.1.1,uid=%d,gid=%d,iocharset=utf8,file_mode=0664,dir_mode=0775,actimeo=30,cache=loose,noserverino,nounix,noperm,nobrl",
		target.UID, target.GID,
	)
	cmd := exec.Command("mount", "-t", "cifs", src, target.Mountpoint, "-o", opts)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mount cifs %s: %s", src, strings.TrimSpace(string(out)))
	}
	return map[string]string{"mountpoint": target.Mountpoint}, nil
}

func (h *Helper) handleUnmountSMB(raw json.RawMessage) (any, error) {
	var req MountSMBRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parâmetros de umount SMB inválidos: %w", err)
	}
	target, err := resolveSMBMount(req)
	if err != nil {
		return nil, err
	}
	if !isCIFSMount(target.Mountpoint) {
		return nil, nil
	}
	cmd := exec.Command("umount", "-l", target.Mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("umount %s: %s", target.Mountpoint, strings.TrimSpace(string(out)))
	}
	return nil, nil
}

func isCIFSMount(mountpoint string) bool {
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
		if fields[1] == want && (fields[2] == "cifs" || fields[2] == "smb3") {
			return true
		}
	}
	return false
}
