package forge

import (
	"strings"
	"time"
)

// LastCommit devolve o commit mais recente de HEAD, ou nil se o bare estiver vazio.
func LastCommit(root, slug string) *CommitInfo {
	items, err := ListCommits(root, slug, "HEAD", "", 1)
	if err != nil || len(items) == 0 {
		return nil
	}
	return &items[0]
}

// PrimaryLanguage é a fatia maior do tree (LanguageStats).
func PrimaryLanguage(root, slug string) string {
	stats, err := LanguageStats(root, slug, "HEAD")
	if err != nil || len(stats) == 0 {
		return ""
	}
	return stats[0].Name
}

// WeeklyCounts soma commits por semana (mais antiga → mais recente).
func WeeklyCounts(root, slug string, weeks int) []int {
	if weeks <= 0 || weeks > 52 {
		weeks = 12
	}
	out := make([]int, weeks)
	dates := commitDates(root, slug, time.Now().UTC().AddDate(0, 0, -7*weeks))
	now := time.Now().UTC()
	start := startOfUTCWeek(now.AddDate(0, 0, -7*(weeks-1)))
	for _, d := range dates {
		idx := int(d.Sub(start).Hours() / (24 * 7))
		if idx >= 0 && idx < weeks {
			out[idx]++
		}
	}
	return out
}

// AuthorDayCounts conta commits do autor (nome) por dia UTC desde `since`.
func AuthorDayCounts(root, slug, author string, since time.Time) map[string]int {
	out := map[string]int{}
	author = strings.TrimSpace(strings.ToLower(author))
	if author == "" {
		return out
	}
	dir, err := RepoPath(root, slug)
	if err != nil || !Exists(root, slug) {
		return out
	}
	bin, err := LookGit()
	if err != nil {
		return out
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "log", "--all", "--format=%aI%x09%an", "--since="+since.UTC().Format(time.RFC3339))
	raw, err := cmd.Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[1])) != author {
			continue
		}
		t, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}
		out[t.UTC().Format("2006-01-02")]++
	}
	return out
}

func commitDates(root, slug string, since time.Time) []time.Time {
	if !Exists(root, slug) {
		return nil
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil
	}
	bin, err := LookGit()
	if err != nil {
		return nil
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "log", "--all", "--format=%aI", "--since="+since.UTC().Format(time.RFC3339))
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}
	var dates []time.Time
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, line)
		if err != nil {
			continue
		}
		dates = append(dates, t.UTC())
	}
	return dates
}

func startOfUTCWeek(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday())
	return time.Date(t.Year(), t.Month(), t.Day()-wd, 0, 0, 0, 0, time.UTC)
}

// ParseCommitTime lê o ISO do git log.
func ParseCommitTime(iso string) *time.Time {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return nil
	}
	u := t.UTC()
	return &u
}
