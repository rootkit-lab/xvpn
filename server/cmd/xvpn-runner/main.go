// xvpn-runner executa jobs CI num peer da malha (Fase 42).
// Não roda no PID do xvpn-server. Só fala com 10.66.66.1:8080 (VPN).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	base := strings.TrimRight(env("XVPN_CI_URL", "http://10.66.66.1:8080"), "/")
	token := os.Getenv("XVPN_RUNNER_TOKEN")
	if token == "" {
		log.Fatal("XVPN_RUNNER_TOKEN é obrigatório")
	}
	gitHost := strings.TrimRight(env("XVPN_GIT_HOST", "https://xgit.corp.ihuull.com"), "/")
	client := &http.Client{Timeout: 2 * time.Minute}
	log.Printf("xvpn-runner polling %s", base)
	for {
		job, err := claim(client, base, token)
		if err != nil {
			log.Printf("claim: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if job == nil {
			time.Sleep(3 * time.Second)
			continue
		}
		log.Printf("job #%d %s %s", job.Number, job.Slug, job.SHA)
		status, out, art := runJob(job, token, gitHost)
		_ = putLog(client, base, token, job.ID, out)
		if art != "" {
			_ = putArtifact(client, base, token, job.ID, art)
		}
		if err := finish(client, base, token, job.ID, status, out); err != nil {
			log.Printf("finish: %v", err)
		}
	}
}

type claimJob struct {
	ID            uint   `json:"id"`
	Number        uint   `json:"number"`
	Slug          string `json:"slug"`
	SHA           string `json:"sha"`
	Ref           string `json:"ref"`
	CloneURL      string `json:"clone_url"`
	PackagesToken string `json:"packages_token"`
}

func claim(c *http.Client, base, token string) (*claimJob, error) {
	req, err := http.NewRequest(http.MethodGet, base+"/api/ci/jobs/next", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, bytes.TrimSpace(b))
	}
	var job claimJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return nil, err
	}
	if job.ID == 0 {
		return nil, nil
	}
	return &job, nil
}

func runJob(job *claimJob, token, gitHost string) (status, logText, artifact string) {
	work, err := os.MkdirTemp("", "xvpn-job-")
	if err != nil {
		return "failed", err.Error(), ""
	}
	defer os.RemoveAll(work)
	src := filepath.Join(work, "src")
	clone := job.CloneURL
	if clone == "" {
		clone = gitHost + "/" + job.Slug
	}
	if u, ok := strings.CutPrefix(clone, "https://"); ok {
		clone = "https://runner:" + token + "@" + u
	}
	var buf bytes.Buffer
	if err := runCmd(&buf, work, "", "git", "clone", "--depth", "50", clone, src); err != nil {
		return "failed", buf.String() + "\n" + err.Error(), ""
	}
	if job.SHA != "" {
		_ = runCmd(&buf, src, "", "git", "fetch", "--depth", "50", "origin", job.SHA)
		if err := runCmd(&buf, src, "", "git", "checkout", job.SHA); err != nil {
			return "failed", buf.String() + "\n" + err.Error(), ""
		}
	}
	script := filepath.Join(src, ".xvpn-ci.sh")
	if _, err := os.Stat(script); err != nil {
		_ = os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o755)
	}
	if err := runCmd(&buf, src, job.PackagesToken, "sh", script); err != nil {
		return "failed", buf.String(), packArtifacts(src)
	}
	return "success", buf.String(), packArtifacts(src)
}

func packArtifacts(src string) string {
	dir := filepath.Join(src, "ci-artifacts")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	out := filepath.Join(os.TempDir(), fmt.Sprintf("xvpn-art-%d.tar", time.Now().UnixNano()))
	if err := runCmd(io.Discard, src, "", "tar", "-cf", out, "ci-artifacts"); err != nil {
		return ""
	}
	return out
}

func putLog(c *http.Client, base, token string, id uint, body string) error {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/ci/jobs/%d/log", base, id), strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

func finish(c *http.Client, base, token string, id uint, status, logText string) error {
	payload, _ := json.Marshal(map[string]string{"status": status, "log": logText})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/ci/jobs/%d/finish", base, id), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}

func putArtifact(c *http.Client, base, token string, id uint, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(path)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return err
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/ci/jobs/%d/artifact", base, id), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

func runCmd(w io.Writer, dir, packagesToken string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = w
	cmd.Stderr = w
	env := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if tok := strings.TrimSpace(packagesToken); tok != "" {
		env = append(env, "XVPN_PACKAGES_TOKEN="+tok)
	}
	cmd.Env = env
	return cmd.Run()
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
