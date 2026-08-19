package forge

import "testing"

func TestWeeklyCountsEmptyRepo(t *testing.T) {
	root := t.TempDir()
	if err := InitBare(root, "lab"); err != nil {
		t.Fatal(err)
	}
	got := WeeklyCounts(root, "lab", 12)
	if len(got) != 12 {
		t.Fatalf("len=%d", len(got))
	}
	if LastCommit(root, "lab") != nil {
		t.Fatal("bare vazio não tem last commit")
	}
	if PrimaryLanguage(root, "lab") != "" {
		t.Fatal("bare vazio não tem language")
	}
}

func TestAuthorDayCountsAfterSeed(t *testing.T) {
	if _, err := LookGit(); err != nil {
		t.Skip("git não está no PATH")
	}
	root := t.TempDir()
	if err := InitBare(root, "lab"); err != nil {
		t.Fatal(err)
	}
	seedTwoBranches(t, root, "lab")
	if LastCommit(root, "lab") == nil {
		t.Fatal("esperava last commit")
	}
	weeks := WeeklyCounts(root, "lab", 12)
	sum := 0
	for _, n := range weeks {
		sum += n
	}
	if sum < 2 {
		t.Fatalf("esperava commits nas semanas: %v", weeks)
	}
}
