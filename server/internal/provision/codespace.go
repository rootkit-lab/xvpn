package provision

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	DefaultCodespaceImage = "gitpod/openvscode-server:1.98.2"
	CodespacePortMin      = 19000
	CodespacePortMax      = 19007
	codespaceMem          = "1536m"
	codespaceCPUs         = "1"
	defaultCodespacesRoot = "/opt/xvpn/data/codespaces"
	defaultGitRoot        = "/opt/xvpn/data/git"
	codespaceCloneHost    = "https://xgit.corp.ihuull.com"
)

var (
	codespaceIDRe    = regexp.MustCompile(`^[a-f0-9]{12}$`)
	codespaceTagRe   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	codespaceTokenRe = regexp.MustCompile(`^[A-Za-z0-9._~-]{16,128}$`)
)

var allowedCodespaceImages = map[string]struct{}{
	"gitpod/openvscode-server": {},
	"codercom/code-server":     {},
}

// CsSpec é o JSON do subcomando cs-apply (stdin).
type CsSpec struct {
	Action          string `json:"action"`
	ID              string `json:"id"`
	Workspace       string `json:"workspace"`
	BarePath        string `json:"bare_path,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Image           string `json:"image,omitempty"`
	Port            uint16 `json:"port,omitempty"`
	CloneURL        string `json:"clone_url,omitempty"`
	GitUser         string `json:"git_user,omitempty"`
	GitToken        string `json:"git_token,omitempty"`
	ConnectionToken string `json:"connection_token,omitempty"`
}

// CsRunner isola git/docker para ApplyCodespace ser testável sem root.
type CsRunner interface {
	LookPath(bin string) (string, error)
	MkdirAll(path string, perm os.FileMode) error
	Git(args ...string) error
	Docker(args ...string) error
	WriteFile(path, content string, perm os.FileMode) error
	RemoveAll(path string) error
	FileExists(path string) (bool, error)
	ReadFile(path string) ([]byte, error)
	ChownRecursive(path string, uid, gid int) error
}

type osCsRunner struct{}

// NewCsRunner devolve o runner de produção (root via sudoers).
func NewCsRunner() CsRunner { return osCsRunner{} }

func (osCsRunner) LookPath(bin string) (string, error) { return exec.LookPath(bin) }

func (osCsRunner) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osCsRunner) Git(args ...string) error {
	bin, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git não encontrado")
	}
	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (osCsRunner) Docker(args ...string) error {
	bin, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker não encontrado")
	}
	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (osCsRunner) WriteFile(path, content string, perm os.FileMode) error {
	return os.WriteFile(path, []byte(content), perm)
}

func (osCsRunner) RemoveAll(path string) error { return os.RemoveAll(path) }

func (osCsRunner) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (osCsRunner) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (osCsRunner) ChownRecursive(path string, uid, gid int) error {
	return filepath.WalkDir(path, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, uid, gid)
	})
}

// ParseCsSpec valida o JSON antes de qualquer syscall.
func ParseCsSpec(raw []byte, codespacesRoot, gitRoot string) (CsSpec, error) {
	if codespacesRoot == "" {
		codespacesRoot = defaultCodespacesRoot
	}
	if gitRoot == "" {
		gitRoot = defaultGitRoot
	}
	var spec CsSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return CsSpec{}, fmt.Errorf("payload de codespace inválido")
	}
	spec.Action = strings.ToLower(strings.TrimSpace(spec.Action))
	spec.ID = strings.ToLower(strings.TrimSpace(spec.ID))
	spec.Workspace = strings.TrimSpace(spec.Workspace)
	spec.BarePath = strings.TrimSpace(spec.BarePath)
	spec.Branch = strings.TrimSpace(spec.Branch)
	spec.Image = strings.TrimSpace(spec.Image)
	spec.CloneURL = strings.TrimSpace(spec.CloneURL)
	spec.GitUser = strings.TrimSpace(spec.GitUser)
	spec.GitToken = strings.TrimSpace(spec.GitToken)
	spec.ConnectionToken = strings.TrimSpace(spec.ConnectionToken)
	switch spec.Action {
	case "create", "start", "stop", "rm":
	default:
		return CsSpec{}, fmt.Errorf("action deve ser create, start, stop ou rm")
	}
	if !codespaceIDRe.MatchString(spec.ID) {
		return CsSpec{}, fmt.Errorf("id inválido")
	}
	ws, err := safeUnderRoot(codespacesRoot, spec.Workspace)
	if err != nil {
		return CsSpec{}, fmt.Errorf("workspace inválido")
	}
	if !strings.HasSuffix(ws, string(os.PathSeparator)+spec.ID+string(os.PathSeparator)+"workspace") &&
		!strings.HasSuffix(ws, "/"+spec.ID+"/workspace") {
		return CsSpec{}, fmt.Errorf("workspace inválido")
	}
	spec.Workspace = ws
	if spec.Action == "create" {
		if !forge.ValidBranchName(spec.Branch) {
			return CsSpec{}, fmt.Errorf("branch inválida")
		}
		bare, err := safeUnderRoot(gitRoot, spec.BarePath)
		if err != nil || !strings.HasSuffix(bare, ".git") {
			return CsSpec{}, fmt.Errorf("bare inválido")
		}
		spec.BarePath = bare
		slug := strings.TrimSuffix(filepath.Base(bare), ".git")
		if !store.ValidProjectSlug(slug) {
			return CsSpec{}, fmt.Errorf("slug inválido")
		}
		want := codespaceCloneHost + "/" + slug
		if spec.CloneURL != want {
			return CsSpec{}, fmt.Errorf("clone_url inválida")
		}
	}
	if spec.Action == "create" || spec.Action == "start" {
		if spec.Image == "" {
			spec.Image = DefaultCodespaceImage
		}
		if err := validateCodespaceImage(spec.Image); err != nil {
			return CsSpec{}, err
		}
		if spec.Port < CodespacePortMin || spec.Port > CodespacePortMax {
			return CsSpec{}, fmt.Errorf("porta fora da faixa %d–%d", CodespacePortMin, CodespacePortMax)
		}
		if !codespaceTokenRe.MatchString(spec.ConnectionToken) {
			return CsSpec{}, fmt.Errorf("connection_token inválido")
		}
	}
	if spec.GitToken != "" && !codespaceTokenRe.MatchString(spec.GitToken) {
		return CsSpec{}, fmt.Errorf("token inválido")
	}
	if spec.GitToken != "" && spec.Action == "start" && !validCodespaceCloneURL(spec.CloneURL) {
		return CsSpec{}, fmt.Errorf("clone_url inválida")
	}
	if spec.GitUser != "" && !ValidUsername(spec.GitUser) && spec.GitUser != "codespace-"+spec.ID {
		return CsSpec{}, fmt.Errorf("git_user inválido")
	}
	return spec, nil
}

func safeUnderRoot(root, raw string) (string, error) {
	if raw == "" || strings.Contains(raw, "\x00") {
		return "", fmt.Errorf("path vazio")
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	clean, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(cleanRoot, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("fora da raiz")
	}
	return clean, nil
}

func validateCodespaceImage(image string) error {
	if strings.ContainsAny(image, " \t\n@") || strings.Contains(image, "..") {
		return fmt.Errorf("imagem inválida")
	}
	repo, tag, ok := strings.Cut(image, ":")
	if !ok || repo == "" || tag == "" {
		return fmt.Errorf("imagem inválida")
	}
	if _, allowed := allowedCodespaceImages[repo]; !allowed {
		return fmt.Errorf("imagem fora da allowlist")
	}
	if !codespaceTagRe.MatchString(tag) {
		return fmt.Errorf("tag inválida")
	}
	return nil
}

// ParseDevcontainerImage lê image de .devcontainer/devcontainer.json se estiver na allowlist.
func ParseDevcontainerImage(raw []byte) (string, error) {
	var doc struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("devcontainer inválido")
	}
	img := strings.TrimSpace(doc.Image)
	if img == "" {
		return "", fmt.Errorf("sem image")
	}
	if err := validateCodespaceImage(img); err != nil {
		return "", err
	}
	return img, nil
}

func validCodespaceCloneURL(u string) bool {
	slug, ok := strings.CutPrefix(u, codespaceCloneHost+"/")
	return ok && store.ValidProjectSlug(slug)
}

func containerName(id string) string { return "xvpn-cs-" + id }

// ApplyCodespace executa create/start/stop/rm. Nunca monta docker.sock.
func ApplyCodespace(r CsRunner, stdin io.Reader, codespacesRoot, gitRoot string) error {
	raw, err := io.ReadAll(io.LimitReader(stdin, 32<<10))
	if err != nil {
		return fmt.Errorf("lendo payload")
	}
	spec, err := ParseCsSpec(raw, codespacesRoot, gitRoot)
	if err != nil {
		return err
	}
	if _, err := r.LookPath("docker"); err != nil && spec.Action != "rm" {
		return fmt.Errorf("docker não encontrado")
	}
	switch spec.Action {
	case "create":
		return csCreate(r, spec)
	case "start":
		if err := csWriteGitCreds(r, spec); err != nil {
			return err
		}
		return r.Docker("start", containerName(spec.ID))
	case "stop":
		return r.Docker("stop", containerName(spec.ID))
	case "rm":
		_ = r.Docker("rm", "-f", containerName(spec.ID))
		return nil
	default:
		return fmt.Errorf("action inválida")
	}
}

func csCreate(r CsRunner, spec CsSpec) error {
	if err := r.MkdirAll(spec.Workspace, 0o750); err != nil {
		return err
	}
	gitDir := filepath.Join(spec.Workspace, ".git")
	exists, err := r.FileExists(gitDir)
	if err != nil {
		return err
	}
	if !exists {
		if err := r.Git("clone", "--no-hardlinks", "--branch", spec.Branch, "--single-branch", spec.BarePath, spec.Workspace); err != nil {
			return err
		}
		if err := r.Git("-C", spec.Workspace, "remote", "set-url", "origin", spec.CloneURL); err != nil {
			return err
		}
	}
	if err := csWriteGitCreds(r, spec); err != nil {
		return err
	}
	if raw, err := r.ReadFile(filepath.Join(spec.Workspace, ".devcontainer", "devcontainer.json")); err == nil {
		if img, err := ParseDevcontainerImage(raw); err == nil {
			spec.Image = img
		}
	}
	if err := r.ChownRecursive(spec.Workspace, 1000, 1000); err != nil {
		return err
	}
	args := dockerRunArgs(spec)
	return r.Docker(args...)
}

func csWriteGitCreds(r CsRunner, spec CsSpec) error {
	if spec.GitToken == "" || spec.GitUser == "" || spec.CloneURL == "" {
		return nil
	}
	cred := strings.TrimPrefix(spec.CloneURL, "https://")
	line := "https://" + spec.GitUser + ":" + spec.GitToken + "@" + cred + "\n"
	if err := r.WriteFile(filepath.Join(spec.Workspace, ".git", "xvpn-credentials"), line, 0o600); err != nil {
		return err
	}
	_ = r.Git("-C", spec.Workspace, "config", "credential.helper", "store --file=.git/xvpn-credentials")
	return nil
}

const openvscodeServerBin = "/home/.openvscode-server/bin/openvscode-server"

func dockerRunArgs(spec CsSpec) []string {
	name := containerName(spec.ID)
	args := []string{
		"run", "-d",
		"--name", name,
		"--memory", codespaceMem,
		"--cpus", codespaceCPUs,
		"--pids-limit", "256",
		"--security-opt", "no-new-privileges",
		"--cap-drop", "ALL",
		"--user", "1000:1000",
		"-p", "127.0.0.1:" + strconv.Itoa(int(spec.Port)) + ":3000",
		"-v", spec.Workspace + ":/home/workspace:rw",
		"--label", "xvpn.codespace=" + spec.ID,
	}
	if strings.HasPrefix(spec.Image, "gitpod/openvscode-server") {
		// A imagem injeta --without-connection-token no ENTRYPOINT.
		args = append(args, "--entrypoint", openvscodeServerBin)
	}
	args = append(args, spec.Image,
		"--host", "0.0.0.0",
		"--port", "3000",
		"--connection-token", spec.ConnectionToken,
		"--default-folder", "/home/workspace",
	)
	return args
}
