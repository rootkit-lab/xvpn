package forge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidBranchName(t *testing.T) {
	if !ValidBranchName("main") || !ValidBranchName("feat/x") {
		t.Fatal("nomes válidos rejeitados")
	}
	if ValidBranchName("release/*") || ValidBranchName("../etc") || ValidBranchName("") {
		t.Fatal("nome perigoso aceito")
	}
}

func TestListAndMergeBranches(t *testing.T) {
	if _, err := LookGit(); err != nil {
		t.Skip("git não está no PATH")
	}
	root := t.TempDir()
	if err := InitBare(root, "lab"); err != nil {
		t.Fatal(err)
	}
	heads, err := ListBranches(root, "lab")
	if err != nil {
		t.Fatal(err)
	}
	if len(heads) != 0 {
		t.Fatalf("bare vazio deveria ter 0 branches, veio %v", heads)
	}

	seedTwoBranches(t, root, "lab")
	heads, err = ListBranches(root, "lab")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(heads, "main") || !contains(heads, "feat") {
		t.Fatalf("branches: %v", heads)
	}
	if !BranchExists(root, "lab", "feat") || BranchExists(root, "lab", "nope") {
		t.Fatal("BranchExists")
	}

	if err := MergeBranches(root, "lab", "feat", "main", "Merge !1"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if err := MergeBranches(root, "lab", "main", "main", ""); err != ErrSameBranch {
		t.Fatalf("mesmo branch: %v", err)
	}
	if err := MergeBranches(root, "lab", "ghost", "main", ""); err != ErrBranchMissing {
		t.Fatalf("ghost: %v", err)
	}
}

func seedTwoBranches(t *testing.T, root, slug string) {
	t.Helper()
	dir, err := RepoPath(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "w")
	runGit(t, "", "clone", dir, work)
	write(t, filepath.Join(work, "README"), "hello\n")
	runGit(t, work, "add", "README")
	runGit(t, work, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "push", "-u", "origin", "main")
	runGit(t, work, "checkout", "-b", "feat")
	write(t, filepath.Join(work, "feat.txt"), "feat\n")
	runGit(t, work, "add", "feat.txt")
	runGit(t, work, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "feat")
	runGit(t, work, "push", "-u", "origin", "feat")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListTreeAndReadBlob(t *testing.T) {
	if _, err := LookGit(); err != nil {
		t.Skip("git não está no PATH")
	}
	root := t.TempDir()
	if err := InitBare(root, "lab"); err != nil {
		t.Fatal(err)
	}
	seedTwoBranches(t, root, "lab")
	ents, err := ListTree(root, "lab", "main", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ents {
		if e.Name == "README" && e.Type == "blob" {
			found = true
		}
	}
	if !found {
		t.Fatalf("README ausente: %+v", ents)
	}
	body, bin, err := ReadBlob(root, "lab", "main", "README")
	if err != nil || bin || body != "hello\n" {
		t.Fatalf("blob: %q bin=%v err=%v", body, bin, err)
	}
	if _, err := ListTree(root, "lab", "main", "../etc"); err == nil {
		t.Fatal("path traversal deveria falhar")
	}
	logs, err := ListCommits(root, "lab", "main", "", 5)
	if err != nil || len(logs) == 0 {
		t.Fatalf("log: %v %v", logs, err)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
