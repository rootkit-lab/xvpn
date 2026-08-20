package forge

import "testing"

func TestCommitFilesEmptyThenUnchanged(t *testing.T) {
	if _, err := LookGit(); err != nil {
		t.Skip("git não está no PATH")
	}
	root := t.TempDir()
	if err := InitBare(root, "lab"); err != nil {
		t.Fatal(err)
	}
	if HasCommits(root, "lab") {
		t.Fatal("bare novo não tem commits")
	}
	res, err := CommitFiles(root, "lab", CommitFilesOpts{
		Files: []FileContent{
			{Path: "README.md", Content: "hello\n"},
			{Path: ".xvpn-ci.sh", Content: "#!/bin/sh\necho ok\n"},
		},
		Message: "chore: seed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA == "" || res.Branch != "main" {
		t.Fatalf("%+v", res)
	}
	if !HasCommits(root, "lab") {
		t.Fatal("esperava HEAD")
	}
	_, err = CommitFiles(root, "lab", CommitFilesOpts{
		Files:   []FileContent{{Path: "README.md", Content: "hello\n"}},
		Ref:     "main",
		Message: "chore: again",
	})
	if err != ErrUnchanged {
		t.Fatalf("unchanged: %v", err)
	}
}
