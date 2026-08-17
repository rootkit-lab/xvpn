package driver

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	MaxExtractBytes int64 = 2 << 30
	MaxExtractFiles       = 4000
)

var (
	ErrNotArchive  = errors.New("arquivo não é um compactado suportado")
	ErrExtractBomb = errors.New("arquivo compactado grande demais")
)

func ArchiveKind(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasSuffix(n, ".zip"):
		return "zip"
	case strings.HasSuffix(n, ".tar.gz"), strings.HasSuffix(n, ".tgz"):
		return "targz"
	default:
		return ""
	}
}

func DestNameForArchive(name string) string {
	n := filepath.Base(name)
	lower := strings.ToLower(n)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return n[:len(n)-7]
	case strings.HasSuffix(lower, ".tgz"):
		return n[:len(n)-4]
	case strings.HasSuffix(lower, ".zip"):
		return n[:len(n)-4]
	default:
		return strings.TrimSuffix(n, filepath.Ext(n))
	}
}

func SafeArchiveRel(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") {
		return "", ErrBadPath
	}
	if name == "" || name == "." {
		return "", ErrBadPath
	}
	if strings.Contains(name, "..") {
		return "", ErrBadPath
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrBadPath
	}
	return clean, nil
}

func mkdirShareExistOK(base, parent, name, root, username string) error {
	err := MkdirShare(base, parent, name, root, username)
	if err == nil || os.IsExist(err) || errors.Is(err, syscall.EEXIST) {
		return nil
	}
	return err
}

func mkdirAllUnder(base, destRoot, rel, root, username string) error {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return nil
	}
	cur := destRoot
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." || strings.ContainsAny(part, `/\`) {
			return ErrBadPath
		}
		if err := mkdirShareExistOK(base, cur, part, root, username); err != nil {
			return err
		}
		cur = filepath.Join(cur, part)
	}
	return nil
}

func writeExtracted(base, destRoot, rel, root, username string, r io.Reader, written *int64) error {
	rel = strings.Trim(rel, "/")
	dir, name := path.Split(rel)
	dir = strings.Trim(dir, "/")
	if name == "" || name == "." || name == ".." {
		return ErrBadPath
	}
	if err := mkdirAllUnder(base, destRoot, dir, root, username); err != nil {
		return err
	}
	parent := destRoot
	if dir != "" {
		parent = filepath.Join(destRoot, filepath.FromSlash(dir))
	}
	dst, err := CreateFileShare(base, parent, name, root, username)
	if err != nil {
		return err
	}
	defer dst.Close()
	remain := MaxExtractBytes - *written
	if remain <= 0 {
		return ErrExtractBomb
	}
	n, err := io.CopyN(dst, r, remain+1)
	*written += n
	if n > remain {
		return ErrExtractBomb
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func ExtractArchive(base, archiveFull, destFull, root, username string) error {
	kind := ArchiveKind(filepath.Base(archiveFull))
	if kind == "" {
		return ErrNotArchive
	}
	f, err := OpenFileNoFollow(base, archiveFull)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	switch kind {
	case "zip":
		return extractZip(base, destFull, root, username, f, st.Size())
	case "targz":
		return extractTarGz(base, destFull, root, username, f)
	default:
		return ErrNotArchive
	}
}

func extractZip(base, destFull, root, username string, f *os.File, size int64) error {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}
	var written int64
	var files int
	for _, zf := range zr.File {
		rel, err := SafeArchiveRel(zf.Name)
		if err != nil {
			return err
		}
		if strings.HasSuffix(zf.Name, "/") || zf.FileInfo().IsDir() {
			if err := mkdirAllUnder(base, destFull, rel, root, username); err != nil {
				return err
			}
			continue
		}
		files++
		if files > MaxExtractFiles {
			return ErrExtractBomb
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeExtracted(base, destFull, rel, root, username, rc, &written)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(base, destFull, root, username string, f *os.File) error {
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var written int64
	var files int
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := SafeArchiveRel(hdr.Name)
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := mkdirAllUnder(base, destFull, rel, root, username); err != nil {
				return err
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		files++
		if files > MaxExtractFiles {
			return ErrExtractBomb
		}
		if err := writeExtracted(base, destFull, rel, root, username, tr, &written); err != nil {
			return err
		}
	}
}
