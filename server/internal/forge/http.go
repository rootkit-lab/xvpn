package forge

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Serve executa git-http-backend (CGI) para o slug. pathInfo é o sufixo
// depois do nome do repo (/info/refs, /git-upload-pack, /git-receive-pack).
func Serve(w http.ResponseWriter, r *http.Request, root, slug, username, pathInfo string) error {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(pathInfo, "/") {
		pathInfo = "/" + pathInfo
	}
	cmd := exec.Command(bin, "http-backend")
	cmd.Dir = filepath.Clean(root)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_PROJECT_ROOT=" + filepath.Clean(root),
		"GIT_HTTP_EXPORT_ALL=1",
		"PATH_INFO=/" + slug + ".git" + pathInfo,
		"REQUEST_METHOD=" + r.Method,
		"QUERY_STRING=" + r.URL.RawQuery,
		"CONTENT_TYPE=" + r.Header.Get("Content-Type"),
		"REMOTE_USER=" + username,
		"REMOTE_ADDR=" + r.RemoteAddr,
	}
	if cl := r.ContentLength; cl > 0 {
		cmd.Env = append(cmd.Env, "CONTENT_LENGTH="+strconv.FormatInt(cl, 10))
	} else if raw := r.Header.Get("Content-Length"); raw != "" {
		cmd.Env = append(cmd.Env, "CONTENT_LENGTH="+raw)
	}
	cmd.Stdin = r.Body
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := writeCGI(w, stdout); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func writeCGI(w http.ResponseWriter, r io.Reader) error {
	br := bufio.NewReader(r)
	status := http.StatusOK
	for {
		line, err := br.ReadString('\n')
		if err != nil && len(line) == 0 {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = http.CanonicalHeaderKey(strings.TrimSpace(k)), strings.TrimSpace(v)
		if strings.EqualFold(k, "Status") {
			if code, _, _ := strings.Cut(v, " "); code != "" {
				if n, err := strconv.Atoi(code); err == nil {
					status = n
				}
			}
			continue
		}
		w.Header().Add(k, v)
	}
	w.WriteHeader(status)
	_, err := io.Copy(w, br)
	return err
}

func ServiceName(query, pathInfo string) string {
	if strings.Contains(query, "git-receive-pack") || strings.Contains(pathInfo, "git-receive-pack") {
		return "git-receive-pack"
	}
	if strings.Contains(query, "git-upload-pack") || strings.Contains(pathInfo, "git-upload-pack") {
		return "git-upload-pack"
	}
	return ""
}

func PathInfo(urlPath, slug string) string {
	slug = NormalizeSlug(slug)
	p := strings.TrimPrefix(urlPath, "/")
	p = strings.TrimPrefix(p, slug+".git")
	p = strings.TrimPrefix(p, slug)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
