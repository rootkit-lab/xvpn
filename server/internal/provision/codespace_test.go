package provision

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeCsRunner struct {
	bins   map[string]string
	git    [][]string
	docker [][]string
	writes map[string]string
	mkdirs []string
}

func newFakeCs() *fakeCsRunner {
	return &fakeCsRunner{
		bins:   map[string]string{"docker": "/usr/bin/docker", "git": "/usr/bin/git"},
		writes: map[string]string{},
	}
}

func (f *fakeCsRunner) LookPath(bin string) (string, error) {
	if p, ok := f.bins[bin]; ok {
		return p, nil
	}
	return "", os.ErrNotExist
}
func (f *fakeCsRunner) MkdirAll(path string, _ os.FileMode) error {
	f.mkdirs = append(f.mkdirs, path)
	return nil
}
func (f *fakeCsRunner) Git(args ...string) error {
	f.git = append(f.git, append([]string{}, args...))
	return nil
}
func (f *fakeCsRunner) Docker(args ...string) error {
	f.docker = append(f.docker, append([]string{}, args...))
	return nil
}
func (f *fakeCsRunner) WriteFile(path, content string, _ os.FileMode) error {
	f.writes[path] = content
	return nil
}
func (f *fakeCsRunner) RemoveAll(string) error { return nil }
func (f *fakeCsRunner) FileExists(string) (bool, error) {
	return false, nil
}
func (f *fakeCsRunner) ReadFile(string) ([]byte, error)       { return nil, os.ErrNotExist }
func (f *fakeCsRunner) ChownRecursive(string, int, int) error { return nil }

func TestParseCsSpec_RejectsUnsafe(t *testing.T) {
	root := t.TempDir()
	csRoot := filepath.Join(root, "codespaces")
	gitRoot := filepath.Join(root, "git")
	id := "aabbccddeeff"
	ws := filepath.Join(csRoot, "alice", "lab", id, "workspace")
	bare := filepath.Join(gitRoot, "lab.git")
	ok := `{
		"action":"create","id":"` + id + `","workspace":"` + ws + `",
		"bare_path":"` + bare + `","branch":"main","port":19000,
		"clone_url":"https://xgit.corp.ihuull.com/lab",
		"image":"gitpod/openvscode-server:1.98.2",
		"connection_token":"tokentokentoken1"
	}`
	if _, err := ParseCsSpec([]byte(ok), csRoot, gitRoot); err != nil {
		t.Fatalf("payload válido: %v", err)
	}
	bads := []string{
		`{"action":"create","id":"../etc","workspace":"` + ws + `","bare_path":"` + bare + `","branch":"main","port":19000,"clone_url":"https://xgit.corp.ihuull.com/lab"}`,
		`{"action":"create","id":"` + id + `","workspace":"/etc/passwd","bare_path":"` + bare + `","branch":"main","port":19000,"clone_url":"https://xgit.corp.ihuull.com/lab"}`,
		`{"action":"create","id":"` + id + `","workspace":"` + ws + `","bare_path":"/opt/xvpn/data/git/../etc","branch":"main","port":19000,"clone_url":"https://xgit.corp.ihuull.com/lab"}`,
		`{"action":"create","id":"` + id + `","workspace":"` + ws + `","bare_path":"` + bare + `","branch":"main","port":22,"clone_url":"https://xgit.corp.ihuull.com/lab"}`,
		`{"action":"create","id":"` + id + `","workspace":"` + ws + `","bare_path":"` + bare + `","branch":"main","port":19000,"clone_url":"https://evil.example/lab"}`,
		`{"action":"create","id":"` + id + `","workspace":"` + ws + `","bare_path":"` + bare + `","branch":"main","port":19000,"clone_url":"https://xgit.corp.ihuull.com/lab","image":"evil/pwn:latest"}`,
		`{"action":"pwn","id":"` + id + `","workspace":"` + ws + `"}`,
	}
	for _, raw := range bads {
		if _, err := ParseCsSpec([]byte(raw), csRoot, gitRoot); err == nil {
			t.Fatalf("deveria rejeitar %s", raw)
		}
	}
}

func TestParseDevcontainerImage_Allowlist(t *testing.T) {
	if _, err := ParseDevcontainerImage([]byte(`{"image":"gitpod/openvscode-server:1.98.2"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDevcontainerImage([]byte(`{"image":"ubuntu:24.04"}`)); err == nil {
		t.Fatal("ubuntu fora da allowlist")
	}
	if _, err := ParseDevcontainerImage([]byte(`{"image":"gitpod/openvscode-server:1.98.2@sha256:abc"}`)); err == nil {
		t.Fatal("digest @ recusado")
	}
}

func TestDockerRunArgs_NoSocketOrPrivileged(t *testing.T) {
	spec := CsSpec{
		ID:              "aabbccddeeff",
		Workspace:       "/opt/xvpn/data/codespaces/alice/lab/aabbccddeeff/workspace",
		Image:           DefaultCodespaceImage,
		Port:            19000,
		ConnectionToken: "tokentokentoken1",
	}
	args := dockerRunArgs(spec)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "docker.sock") || strings.Contains(joined, "privileged") || strings.Contains(joined, "network=host") {
		t.Fatalf("flags proibidas: %s", joined)
	}
	if strings.Contains(joined, "without-connection-token") {
		t.Fatalf("IDE na loopback precisa de connection token: %s", joined)
	}
	if !strings.Contains(joined, "--entrypoint "+openvscodeServerBin) {
		t.Fatalf("entrypoint da imagem conflita com o token: %s", joined)
	}
	if !strings.Contains(joined, "--connection-token tokentokentoken1") {
		t.Fatalf("connection token ausente: %s", joined)
	}
	if !strings.Contains(joined, "127.0.0.1:19000:3000") {
		t.Fatalf("publish deve ser loopback: %s", joined)
	}
	if !strings.Contains(joined, "no-new-privileges") || !strings.Contains(joined, "cap-drop") {
		t.Fatalf("hardening em falta: %s", joined)
	}
}

func TestApplyCodespace_CreateClonesBareNotWorktree(t *testing.T) {
	root := t.TempDir()
	csRoot := filepath.Join(root, "codespaces")
	gitRoot := filepath.Join(root, "git")
	id := "aabbccddeeff"
	ws := filepath.Join(csRoot, "alice", "lab", id, "workspace")
	bare := filepath.Join(gitRoot, "lab.git")
	payload := `{
		"action":"create","id":"` + id + `","workspace":"` + ws + `",
		"bare_path":"` + bare + `","branch":"main","port":19003,
		"clone_url":"https://xgit.corp.ihuull.com/lab",
		"git_user":"codespace-` + id + `","git_token":"tokentokentoken1",
		"connection_token":"tokentokentoken1"
	}`
	f := newFakeCs()
	if err := ApplyCodespace(f, strings.NewReader(payload), csRoot, gitRoot); err != nil {
		t.Fatal(err)
	}
	if len(f.git) == 0 || f.git[0][0] != "clone" {
		t.Fatalf("esperava git clone, veio %v", f.git)
	}
	joinedGit := strings.Join(f.git[0], " ")
	if strings.Contains(joinedGit, "worktree") {
		t.Fatal("não pode ser worktree")
	}
	if !strings.Contains(joinedGit, "--no-hardlinks") {
		t.Fatal("clone local sem --no-hardlinks compartilha inode com o bare")
	}
	if f.git[0][len(f.git[0])-2] != bare {
		t.Fatalf("clone deve ser do bare: %v", f.git[0])
	}
	if len(f.docker) != 1 {
		t.Fatalf("docker: %v", f.docker)
	}
	joined := strings.Join(f.docker[0], " ")
	if strings.Contains(joined, "docker.sock") || strings.Contains(joined, bare) {
		t.Fatalf("container não monta bare nem socket: %s", joined)
	}
}

func TestApplyCodespace_StartRewritesCreds(t *testing.T) {
	root := t.TempDir()
	csRoot := filepath.Join(root, "codespaces")
	gitRoot := filepath.Join(root, "git")
	id := "aabbccddeeff"
	ws := filepath.Join(csRoot, "alice", "lab", id, "workspace")
	payload := `{
		"action":"start","id":"` + id + `","workspace":"` + ws + `",
		"port":19003,"image":"gitpod/openvscode-server:1.98.2",
		"clone_url":"https://xgit.corp.ihuull.com/lab",
		"git_user":"codespace-` + id + `","git_token":"rotatedtoken0001",
		"connection_token":"tokentokentoken1"
	}`
	f := newFakeCs()
	if err := ApplyCodespace(f, strings.NewReader(payload), csRoot, gitRoot); err != nil {
		t.Fatal(err)
	}
	if len(f.docker) != 1 || f.docker[0][0] != "start" {
		t.Fatalf("esperava docker start: %v", f.docker)
	}
	cred := filepath.Join(ws, ".git", "xvpn-credentials")
	if !strings.Contains(f.writes[cred], "rotatedtoken0001") {
		t.Fatalf("credencial não rotacionada: %v", f.writes)
	}
}

func TestChownRecursive_DoesNotFollowSymlink(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("lchown para outro uid exige root")
	}
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, "workspace")
	if err := os.Mkdir(ws, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(ws, "link")); err != nil {
		t.Fatal(err)
	}
	if err := (osCsRunner{}).ChownRecursive(ws, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("stat")
	}
	if st.Uid == 1000 {
		t.Fatal("chown seguiu o symlink e alterou o alvo")
	}
}
