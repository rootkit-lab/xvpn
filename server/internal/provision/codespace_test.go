package provision

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeCsRunner struct {
	bins      map[string]string
	git       [][]string
	docker    [][]string
	host      [][]string
	hostFail  map[string]error
	writes    map[string]string
	mkdirs    []string
	inspectIP string
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
func (f *fakeCsRunner) DockerOutput(args ...string) (string, error) {
	f.docker = append(f.docker, append([]string{}, args...))
	if f.inspectIP != "" {
		return f.inspectIP, nil
	}
	return "172.17.0.2", nil
}
func (f *fakeCsRunner) HostCmd(name string, args ...string) error {
	f.host = append(f.host, append([]string{name}, args...))
	key := strings.Join(append([]string{name}, args...), " ")
	if err, ok := f.hostFail[key]; ok {
		return err
	}
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
		`{"action":"create","id":"` + id + `","workspace":"` + ws + `","bare_path":"` + bare + `","branch":"main","port":19000,"clone_url":"https://xgit.corp.ihuull.com/lab","git_author":"alice","git_email":"eve@evil.com","image":"gitpod/openvscode-server:1.98.2","connection_token":"tokentokentoken1"}`,
	}
	for _, raw := range bads {
		if _, err := ParseCsSpec([]byte(raw), csRoot, gitRoot); err == nil {
			t.Fatalf("deveria rejeitar %s", raw)
		}
	}
}

func TestParseDevcontainerImage_Allowlist(t *testing.T) {
	if _, err := ParseDevcontainerImage([]byte(`{"image":"ihuull/codespace:1.98.2"}`)); err != nil {
		t.Fatal(err)
	}
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

func TestParseDevcontainer_Settings(t *testing.T) {
	raw := []byte(`{
		"image":"ihuull/codespace:1.98.2",
		"customizations":{"vscode":{"settings":{"workbench.colorTheme":"ihuull Dark"}}}
	}`)
	dc, err := ParseDevcontainer(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dc.Settings["workbench.colorTheme"] != "ihuull Dark" {
		t.Fatalf("settings: %#v", dc.Settings)
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
	if !strings.Contains(joined, ":"+codespaceProjectDir+":rw") {
		t.Fatalf("clone deve montar em %s: %s", codespaceProjectDir, joined)
	}
	if strings.Contains(joined, ":/home/workspace:rw") {
		t.Fatalf("HOME do IDE não pode ser o clone: %s", joined)
	}
	if !strings.Contains(joined, "--default-folder "+codespaceProjectDir) {
		t.Fatalf("default-folder deve ser o clone: %s", joined)
	}
	if !strings.Contains(joined, machineSettingsHostPath(spec.Workspace)+":"+codespaceMachineDest+":ro") {
		t.Fatalf("settings Machine ausentes: %s", joined)
	}
	if !strings.Contains(joined, "no-new-privileges") || !strings.Contains(joined, "cap-drop") {
		t.Fatalf("hardening em falta: %s", joined)
	}
	if !strings.Contains(joined, "--add-host xgit.corp.ihuull.com:10.66.66.1") {
		t.Fatal("codespace precisa resolver xgit.corp sem o DNS da VPC")
	}
	if !strings.Contains(joined, "--add-host cs-aabbccddeeff.corp.ihuull.com:10.66.66.1") {
		t.Fatal("codespace precisa resolver o próprio cs-* para o LLM")
	}
}

func TestDockerRunArgs_InjectsEnvOmitsLLM(t *testing.T) {
	spec := CsSpec{
		ID:              "aabbccddeeff",
		Workspace:       "/opt/xvpn/data/codespaces/alice/lab/aabbccddeeff/workspace",
		Image:           DefaultCodespaceImage,
		Port:            19000,
		ConnectionToken: "tokentokentoken1",
		Env:             map[string]string{"APP_URL": "https://xgit.corp"},
	}
	joined := strings.Join(dockerRunArgs(spec), " ")
	if !strings.Contains(joined, "--env-file "+runtimeEnvHostPath(spec.Workspace)) {
		t.Fatalf("env-file ausente: %s", joined)
	}
	if strings.Contains(joined, "https://xgit.corp") || strings.Contains(joined, "-e APP_URL=") {
		t.Fatalf("valor de ENV não pode ir no argv: %s", joined)
	}
	if err := validateCodespaceEnv(map[string]string{"XCS_LLM_KEY": "sk"}); err == nil {
		t.Fatal("key LLM não pode ir ao container")
	}
	if err := validateCodespaceEnv(map[string]string{"PATH": "/bin"}); err == nil {
		t.Fatal("PATH bloqueado")
	}
	if err := validateCodespaceEnv(map[string]string{"NODE_OPTIONS": "--require=/tmp/x"}); err == nil {
		t.Fatal("NODE_OPTIONS bloqueado")
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
		"git_author":"alice","git_email":"alice@corp.ihuull.com",
		"connection_token":"tokentokentoken1",
		"env":{"APP_URL":"https://xgit.corp"}
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
	run := 0
	for _, d := range f.docker {
		if len(d) > 0 && d[0] == "run" {
			run++
			joined := strings.Join(d, " ")
			if strings.Contains(joined, "docker.sock") || strings.Contains(joined, bare) {
				t.Fatalf("container não monta bare nem socket: %s", joined)
			}
		}
	}
	if run != 1 {
		t.Fatalf("docker run: %v", f.docker)
	}
	if _, ok := f.writes[filepath.Join(ws, ".vscode", "settings.json")]; ok {
		t.Fatal("settings do IDE não podem ir no clone")
	}
	settings := machineSettingsHostPath(ws)
	if !strings.Contains(f.writes[settings], "ihuull Dark") {
		t.Fatalf("tema não gravado: %v", f.writes)
	}
	if !strings.Contains(f.writes[settings], `"ihuull.codespace.id": "aabbccddeeff"`) {
		t.Fatal("Machine settings devem gravar o id do codespace")
	}
	if !strings.Contains(f.writes[settings], "https://cs-aabbccddeeff.corp.ihuull.com") {
		t.Fatal("Machine settings devem gravar a origin cs-*")
	}
	if !strings.Contains(f.writes[settings], "SetupWeb") {
		t.Fatal("Welcome builtin SetupWeb deve ser ocultado nas Machine settings")
	}
	if !strings.Contains(f.writes[settings], "chat.commandCenter.enabled") {
		t.Fatal("Machine settings devem desligar o chat nativo")
	}
	if !strings.Contains(f.writes[settings], `"ihuull.codespace.gitName": "alice"`) {
		t.Fatal("Machine settings devem gravar a identidade Git")
	}
	var gitFlat []string
	for _, g := range f.git {
		gitFlat = append(gitFlat, strings.Join(g, " "))
	}
	joinedAllGit := strings.Join(gitFlat, " | ")
	if !strings.Contains(joinedAllGit, "user.name") || !strings.Contains(joinedAllGit, "alice") {
		t.Fatalf("clone precisa de user.name: %v", f.git)
	}
	envFile := runtimeEnvHostPath(ws)
	if !strings.Contains(f.writes[envFile], "APP_URL=https://xgit.corp") {
		t.Fatalf("env-file não gravado: %v", f.writes)
	}
	for _, d := range f.docker {
		if len(d) > 0 && d[0] == "run" && strings.Contains(strings.Join(d, " "), "https://xgit.corp") {
			t.Fatalf("valor de ENV no argv: %v", d)
		}
	}
}

func TestDefaultCodespaceSettings_HidesBuiltinWelcome(t *testing.T) {
	s := defaultCodespaceSettings()
	if s["workbench.startupEditor"] != "welcomePage" {
		t.Fatalf("startupEditor: %v", s["workbench.startupEditor"])
	}
	hide := "experiments.override.gettingStarted.overrideCategory.SetupWeb.when"
	if s[hide] != "false" {
		t.Fatalf("SetupWeb deve ficar oculto: %v", s[hide])
	}
	if s["chat.commandCenter.enabled"] != false {
		t.Fatal("command center do chat nativo deve ficar off")
	}
	if s["workbench.secondarySideBar.defaultVisibility"] != "visible" {
		t.Fatal("secondary sidebar deve ficar visible para o chat ihuull")
	}
}

func TestCodespaceAssistantExtension_HasGenerateCommit(t *testing.T) {
	pkg, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "vscode-codespace", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pkg), "ihuull.generateCommitMessage") {
		t.Fatal("extensão precisa do comando generate commit")
	}
	js, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "vscode-codespace", "extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "/api/xcodespaces/llm/commit-message") {
		t.Fatal("generate commit deve chamar o proxy")
	}
	if strings.Contains(string(js), `fetch(path,`) {
		t.Fatal("fetch relativo quebra no Node do extension host")
	}
	if !strings.Contains(string(js), "https://cs-") || !strings.Contains(string(js), "xvpn-credentials") {
		t.Fatal("extensão precisa de origin absoluta e do token Git do codespace")
	}
	if !strings.Contains(string(pkg), `"id": "ihuull.agentView"`) {
		t.Fatal("extensão precisa da view do agente")
	}
	if !strings.Contains(string(pkg), `"workbench.panel.chat"`) || strings.Contains(string(pkg), `"activitybar"`) {
		t.Fatal("OpenVSCode 1.98 ignora secondarySidebar — o chat mora no container Chat da direita")
	}
	if !strings.Contains(string(pkg), `"id": "ihuull-ports"`) || strings.Contains(string(pkg), `"workbench.panel":`) {
		t.Fatal("Ports deve ser viewsContainers.panel — workbench.panel cai no Explorer")
	}
	if !strings.Contains(string(js), "/api/xcodespaces/llm/models") {
		t.Fatal("chat precisa listar modelos no proxy")
	}
	if !strings.Contains(string(js), "user.email") {
		t.Fatal("extensão deve gravar user.name/email do dono")
	}
	banned, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "vscode-codespace", "banned.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(banned), "GitHub.copilot") || !strings.Contains(string(banned), "Continue.continue") {
		t.Fatal("ban list deve incluir Copilot e Continue")
	}
	if strings.Contains(string(banned), "closeAuxiliaryBar") {
		t.Fatal("não fechar a auxiliary bar — o chat ihuull mora lá")
	}
	if !strings.Contains(string(banned), "focusAuxiliaryBar") {
		t.Fatal("activate deve focar a auxiliary bar")
	}
}

func TestCodespaceAgentSandbox(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "shared", "vscode-codespace")
	cmd := exec.Command("node", "--test", "sandbox.test.js", "tools.test.js")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox.test.js: %v\n%s", err, out)
	}
}

func TestCodespaceThemeExtension_OpensIhuullWalkthrough(t *testing.T) {
	pkg, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "vscode-theme", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(pkg)
	if !strings.Contains(text, `"main": "./extension.js"`) {
		t.Fatal("tema precisa de extension.js para abrir o walkthrough XCODESPACES")
	}
	if !strings.Contains(text, "onStartupFinished") {
		t.Fatal("activation onStartupFinished ausente")
	}
	js, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "vscode-theme", "extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "ihuull.ihuull-theme#ihuull.codespace") {
		t.Fatal("extension.js deve abrir o walkthrough ihuull")
	}
}

func TestSampleTeste_HasPlaygroundFiles(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "codespace", "sample-teste")
	for _, rel := range []string{
		"README.md",
		"go.mod",
		"cmd/hello/main.go",
		"cmd/hello/main_test.go",
		"web/index.mjs",
		"web/package.json",
		"scripts/check.sh",
		".devcontainer/devcontainer.json",
		".gitignore",
		".vscode/tasks.json",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("sample-teste sem %s: %v", rel, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "XCODESPACES") {
		t.Fatal("README do teste deve ser o checklist do codespace")
	}
}

func TestCodespaceDockerfile_NoSocketOrPrivileged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "codespace", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "/var/run/docker.sock") || strings.Contains(text, "--privileged") {
		t.Fatal("Dockerfile do codespace não pode ter socket nem privileged")
	}
	if !strings.Contains(text, "ihuull.theme") {
		t.Fatal("tema ihuull deve ir na imagem")
	}
	if !strings.Contains(text, "ihuull.codespace") {
		t.Fatal("extensão ihuull.codespace deve ir na imagem")
	}
	if !strings.Contains(text, "ripgrep") {
		t.Fatal("imagem precisa de rg para a tool grep do agente")
	}
	if !strings.Contains(text, "python3") {
		t.Fatal("imagem precisa de python3 para o agente")
	}
	if !strings.Contains(text, "gitignore-global") {
		t.Fatal("gitignore global deve ir na imagem")
	}
	gi, err := os.ReadFile(filepath.Join("..", "..", "deploy", "codespace", "gitignore-global"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), ".cursor/agent/") {
		t.Fatal("gitignore-global deve ignorar logs do agente")
	}
	if !strings.Contains(text, "xcs-analyze") {
		t.Fatal("analyzer Go deve ir na imagem")
	}
	if !strings.Contains(text, "install-ovsx.sh") {
		t.Fatal("Open VSX deve ser bakeado via install-ovsx.sh")
	}
	if strings.Contains(text, "marketplace.visualstudio.com") {
		t.Fatal("sem Marketplace Microsoft")
	}
	ovsx, err := os.ReadFile(filepath.Join("..", "..", "deploy", "codespace", "install-ovsx.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(ovsx)
	for _, id := range []string{"golang.go", "dbaeumer.vscode-eslint", "esbenp.prettier-vscode@11.0.0", "yzhang.markdown-all-in-one", "redhat.vscode-yaml"} {
		if !strings.Contains(script, id) {
			t.Fatalf("Open VSX sem %s", id)
		}
	}
	if !strings.Contains(script, "https://open-vsx.org/") {
		t.Fatal("VSIX só do Open VSX")
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
