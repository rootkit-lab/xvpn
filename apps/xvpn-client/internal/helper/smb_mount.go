package helper

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// serverVPNAddress é o IP do Samba no túnel — AGENTS.md / PLAN.md §5.
const serverVPNAddress = "10.66.66.1"

var smbShareName = regexp.MustCompile(`^(shared|home-[a-z][a-z0-9_-]{0,31})$`)

// MountSMBRequest é o parâmetro de mount_smb. Host/share vêm da GUI;
// uid/gid e o mountpoint vêm do peer Unix (SO_PEERCRED), nunca deste JSON.
type MountSMBRequest struct {
	Host  string `json:"host"`
	Share string `json:"share"`
}

type mountSMBTarget struct {
	Host       string
	Share      string
	Mountpoint string
	Home       string
	UID        int
	GID        int
}

func shareFolderName(share string) string {
	if share == "shared" {
		return "Compartilhado"
	}
	if strings.HasPrefix(share, "home-") {
		return "Meus arquivos"
	}
	return share
}

func resolveSMBMount(req MountSMBRequest, uid, gid int) (mountSMBTarget, error) {
	host := strings.TrimSpace(req.Host)
	share := strings.TrimSpace(req.Share)
	if host != serverVPNAddress {
		return mountSMBTarget{}, fmt.Errorf("host SMB inválido — só %s", serverVPNAddress)
	}
	if !smbShareName.MatchString(share) {
		return mountSMBTarget{}, fmt.Errorf("share SMB inválido")
	}
	if uid < 1 || gid < 0 {
		return mountSMBTarget{}, fmt.Errorf("uid/gid inválidos")
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return mountSMBTarget{}, fmt.Errorf("usuário %d: %w", uid, err)
	}
	if u.HomeDir == "" || u.HomeDir == "/" {
		return mountSMBTarget{}, fmt.Errorf("home do uid %d inválida", uid)
	}
	mountpoint := filepath.Join(u.HomeDir, "XVPN", shareFolderName(share))
	return mountSMBTarget{
		Host:       host,
		Share:      share,
		Mountpoint: mountpoint,
		Home:       u.HomeDir,
		UID:        uid,
		GID:        gid,
	}, nil
}

// unescapeProcMount decodifica escapes octais de /proc/mounts (\040 = espaço).
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

func pathUnderHome(realPath, home string) bool {
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		realHome = filepath.Clean(home)
	}
	rel, err := filepath.Rel(realHome, realPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
