// Package driver resolve paths do XDriver (pastas Samba) sem FileBrowser.
package driver

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	ErrBadPath = errors.New("caminho inválido")
	ErrBadRoot = errors.New("raiz inválida")
	ErrBadUser = errors.New("usuário inválido")
)

type Roots struct {
	SharedDir string
	HomeRoot  string
}

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

func validUsername(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// Resolve devolve o path absoluto dentro de home/<user>/files ou shared.
func (r Roots) Resolve(username, root, rel string) (string, error) {
	if !validUsername(username) {
		return "", ErrBadUser
	}
	if root != "home" && root != "shared" {
		return "", ErrBadRoot
	}
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "/")
	if strings.Contains(rel, "..") {
		return "", ErrBadPath
	}
	base := filepath.Clean(r.SharedDir)
	if root == "home" {
		base = filepath.Clean(filepath.Join(r.HomeRoot, username, "files"))
	}
	full := filepath.Clean(filepath.Join(base, rel))
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", ErrBadPath
	}
	return full, nil
}

func RelFrom(base, full string) string {
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

func List(dir string) ([]Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	out := make([]Entry, 0, len(ents))
	for _, e := range ents {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, Entry{
			Name:    e.Name(),
			Path:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}
	return out, nil
}
