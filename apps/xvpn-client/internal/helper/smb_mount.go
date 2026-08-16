package helper

import (
	"fmt"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// serverVPNAddress é o IP do Samba no túnel — AGENTS.md / PLAN.md §5.
const serverVPNAddress = "10.66.66.1"

var smbShareName = regexp.MustCompile(`^(shared|home-[a-z][a-z0-9_-]{0,31})$`)

// MountSMBRequest é o parâmetro de mount_smb. O helper resolve o
// mountpoint a partir do uid (não confia em path vindo da GUI).
type MountSMBRequest struct {
	Host  string `json:"host"`
	Share string `json:"share"`
	UID   int    `json:"uid"`
	GID   int    `json:"gid"`
}

type mountSMBTarget struct {
	Host       string
	Share      string
	Mountpoint string
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

func resolveSMBMount(req MountSMBRequest) (mountSMBTarget, error) {
	host := strings.TrimSpace(req.Host)
	share := strings.TrimSpace(req.Share)
	if host != serverVPNAddress {
		return mountSMBTarget{}, fmt.Errorf("host SMB inválido — só %s", serverVPNAddress)
	}
	if !smbShareName.MatchString(share) {
		return mountSMBTarget{}, fmt.Errorf("share SMB inválido")
	}
	if req.UID < 1 || req.GID < 0 {
		return mountSMBTarget{}, fmt.Errorf("uid/gid inválidos")
	}
	u, err := user.LookupId(strconv.Itoa(req.UID))
	if err != nil {
		return mountSMBTarget{}, fmt.Errorf("usuário %d: %w", req.UID, err)
	}
	if u.HomeDir == "" || u.HomeDir == "/" {
		return mountSMBTarget{}, fmt.Errorf("home do uid %d inválida", req.UID)
	}
	mountpoint := filepath.Join(u.HomeDir, "XVPN", shareFolderName(share))
	return mountSMBTarget{
		Host:       host,
		Share:      share,
		Mountpoint: mountpoint,
		UID:        req.UID,
		GID:        req.GID,
	}, nil
}
