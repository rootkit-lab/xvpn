// Package driver resolve paths do XDriver (pastas Samba) sem FileBrowser.
package driver

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// RejectSymlinks recusa qualquer componente existente de full que seja
// symlink, para mkdir/upload/chown não saírem da raiz do share.
func RejectSymlinks(base, full string) error {
	base = filepath.Clean(base)
	full = filepath.Clean(full)
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return ErrBadPath
	}
	if err := rejectIfSymlink(base); err != nil {
		if os.IsNotExist(err) {
			return ErrBadPath
		}
		return err
	}
	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ErrBadPath
	}
	if rel == "." {
		return nil
	}
	cur := base
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		err := rejectIfSymlink(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func rejectIfSymlink(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return ErrBadPath
	}
	return nil
}

const noFollowDir = syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC

// OpenDirNoFollow abre full andando base→folha com openat(O_NOFOLLOW).
// Um pai trocado por symlink entre o Lstat e o mkdir não é seguido.
func OpenDirNoFollow(base, full string) (int, error) {
	base = filepath.Clean(base)
	full = filepath.Clean(full)
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return -1, ErrBadPath
	}
	fd, err := syscall.Open(base, noFollowDir, 0)
	if err != nil {
		return -1, err
	}
	if full == base {
		return fd, nil
	}
	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		syscall.Close(fd)
		return -1, ErrBadPath
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." || strings.ContainsAny(part, `/\`) {
			syscall.Close(fd)
			return -1, ErrBadPath
		}
		next, err := syscall.Openat(fd, part, noFollowDir, 0)
		syscall.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	return fd, nil
}

func shareOwner(root, username string) (name string, uid, gid int, err error) {
	name = username
	if root == "shared" {
		name = "xvpn-shared"
	}
	if !validUsername(name) {
		return "", 0, 0, ErrBadUser
	}
	u, err := user.Lookup(name)
	if err != nil {
		return "", 0, 0, err
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return "", 0, 0, err
	}
	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		return "", 0, 0, err
	}
	return name, uid, gid, nil
}

func applyShareACL(fd int, root, username string) error {
	name, uid, gid, err := shareOwner(root, username)
	if err != nil {
		return err
	}
	proc := "/proc/self/fd/" + strconv.Itoa(fd)
	spec := "u:xvpn:rwx,u:" + name + ":rwx"
	_ = exec.Command("setfacl", "-m", spec, proc).Run()
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Mode&syscall.S_IFMT == syscall.S_IFDIR {
		_ = exec.Command("setfacl", "-d", "-m", spec, proc).Run()
	}
	return syscall.Fchown(fd, uid, gid)
}

func chownShareAt(dirfd int, name, root, username string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ErrBadPath
	}
	fd, err := syscall.Openat(dirfd, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	return applyShareACL(fd, root, username)
}

// MkdirShare cria name dentro de parent sem seguir symlink em nenhum passo.
func MkdirShare(base, parent, name, root, username string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ErrBadPath
	}
	dirfd, err := OpenDirNoFollow(base, parent)
	if err != nil {
		return err
	}
	defer syscall.Close(dirfd)
	if err := syscall.Mkdirat(dirfd, name, 0o775); err != nil {
		return err
	}
	return chownShareAt(dirfd, name, root, username)
}

// CreateFileShare cria/truncates o arquivo sob dir com O_NOFOLLOW em cada passo.
func CreateFileShare(base, dir, name, root, username string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, ErrBadPath
	}
	dirfd, err := OpenDirNoFollow(base, dir)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(dirfd)
	fd, err := syscall.Openat(dirfd, name, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o664)
	if err != nil {
		return nil, err
	}
	_ = applyShareACL(fd, root, username)
	return os.NewFile(uintptr(fd), name), nil
}

// ChownShare passa o arquivo/pasta criado pelo Drive para o force user
// do Samba (dono do home ou xvpn-shared) e mantém ACL rwx do xvpn.
// Sem isso o GVFS/CIFS do dono toma permission denied (os error 13).
// Abre com O_NOFOLLOW e aplica ACL/chown no fd — um symlink plantado
// no share não recebe setfacl no referente.
func ChownShare(path, root, username string) error {
	name := username
	if root == "shared" {
		name = "xvpn-shared"
	}
	if !validUsername(name) {
		return ErrBadUser
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	return applyShareACL(fd, root, username)
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
