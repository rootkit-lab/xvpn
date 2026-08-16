//go:build linux

package helper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rootkit-lab/xvpn/client/internal/ipc"
	"golang.org/x/sys/unix"
)

func (h *Helper) handleMountSMB(raw json.RawMessage, peer ipc.Peer) (any, error) {
	var req MountSMBRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parâmetros de mount SMB inválidos: %w", err)
	}
	target, err := resolveSMBMount(req, peer.UID, peer.GID)
	if err != nil {
		return nil, err
	}
	fd, err := prepareMountpoint(target)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	if isCIFSMount(target.Mountpoint) {
		return map[string]string{"mountpoint": target.Mountpoint}, nil
	}
	if _, err := exec.LookPath("mount.cifs"); err != nil {
		return nil, fmt.Errorf("cifs-utils não instalado (mount.cifs) — o cliente cai no GVFS")
	}
	src := fmt.Sprintf("//%s/%s", target.Host, target.Share)
	opts := fmt.Sprintf(
		"guest,vers=3.1.1,uid=%d,gid=%d,iocharset=utf8,file_mode=0600,dir_mode=0700,actimeo=30,cache=loose,noserverino,nounix,nobrl",
		target.UID, target.GID,
	)
	// /proc/self/fd/N prende o inode (O_NOFOLLOW) — mount não segue symlink.
	dest := fmt.Sprintf("/proc/self/fd/%d", fd)
	cmd := exec.Command("mount", "-t", "cifs", src, dest, "-o", opts)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mount cifs %s: %s", src, strings.TrimSpace(string(out)))
	}
	return map[string]string{"mountpoint": target.Mountpoint}, nil
}

func (h *Helper) handleUnmountSMB(raw json.RawMessage, peer ipc.Peer) (any, error) {
	var req MountSMBRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parâmetros de umount SMB inválidos: %w", err)
	}
	target, err := resolveSMBMount(req, peer.UID, peer.GID)
	if err != nil {
		return nil, err
	}
	if !isCIFSMount(target.Mountpoint) {
		return nil, nil
	}
	fd, err := openMountpoint(target)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	dest := fmt.Sprintf("/proc/self/fd/%d", fd)
	cmd := exec.Command("umount", "-l", "--no-canonicalize", "--", dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("umount %s: %s", target.Mountpoint, strings.TrimSpace(string(out)))
	}
	return nil, nil
}

func openMountpoint(target mountSMBTarget) (int, error) {
	homeFD, err := unix.Open(target.Home, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		return -1, fmt.Errorf("abrindo home %s: %w", target.Home, err)
	}
	defer unix.Close(homeFD)
	xvpnFD, err := unix.Openat(homeFD, "XVPN", unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY, 0)
	if err != nil {
		return -1, fmt.Errorf("abrindo XVPN: %w", err)
	}
	defer unix.Close(xvpnFD)
	leaf := shareFolderName(target.Share)
	fd, err := unix.Openat(xvpnFD, leaf, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY, 0)
	if err != nil {
		return -1, fmt.Errorf("abrindo %s: %w", leaf, err)
	}
	return fd, nil
}

func prepareMountpoint(target mountSMBTarget) (int, error) {
	homeFD, err := unix.Open(target.Home, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		return -1, fmt.Errorf("abrindo home %s: %w", target.Home, err)
	}
	defer unix.Close(homeFD)

	xvpnFD, err := mkdirOpenat(homeFD, "XVPN", target.UID, target.GID)
	if err != nil {
		return -1, fmt.Errorf("criando XVPN: %w", err)
	}
	defer unix.Close(xvpnFD)

	leaf := shareFolderName(target.Share)
	leafFD, err := mkdirOpenat(xvpnFD, leaf, target.UID, target.GID)
	if err != nil {
		return -1, fmt.Errorf("criando %s: %w", leaf, err)
	}
	return leafFD, nil
}

func mkdirOpenat(dirfd int, name string, uid, gid int) (int, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return -1, fmt.Errorf("nome de diretório inválido")
	}
	var st unix.Stat_t
	err := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil && st.Mode&unix.S_IFMT == unix.S_IFLNK {
		if err := unix.Unlinkat(dirfd, name, 0); err != nil {
			return -1, fmt.Errorf("removendo symlink %s: %w", name, err)
		}
	}
	if err := unix.Mkdirat(dirfd, name, 0o700); err != nil && err != unix.EEXIST {
		return -1, err
	}
	fd, err := unix.Openat(dirfd, name, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY, 0)
	if err != nil {
		return -1, err
	}
	if err := unix.Fchown(fd, uid, gid); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("fchown %s: %w", name, err)
	}
	if err := unix.Fchmod(fd, 0o700); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("fchmod %s: %w", name, err)
	}
	return fd, nil
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
		got := strings.TrimRight(unescapeProcMount(fields[1]), "/")
		if got == want && (fields[2] == "cifs" || fields[2] == "smb3") {
			return true
		}
	}
	return false
}
