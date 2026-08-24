package gitssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// Service executa git-(upload|receive)-pack para sessões SSH do forge.
type Service struct {
	Access *Access
}

// Run atende SSH_ORIGINAL_COMMAND para o usuário do painel indicado.
func (s *Service) Run(panelUser, originalCmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd, err := ParseOriginalCommand(originalCmd)
	if err != nil {
		return err
	}
	user, err := s.Access.LoadUser(panelUser)
	if err != nil {
		return ErrNotFound
	}
	org, slug, _ := forge.SplitRepo(cmd.Repo)
	proj, ok := s.Access.FindProject(org, slug)
	if !ok {
		return ErrNotFound
	}
	if !s.Access.CanGitRead(user, proj) {
		return ErrNotFound
	}
	gitDir, err := forge.RepoPath(s.Access.GitDir, cmd.Repo)
	if err != nil || !forge.Exists(s.Access.GitDir, cmd.Repo) {
		return ErrNotFound
	}
	bin, err := forge.LookGit()
	if err != nil {
		return err
	}

	switch cmd.Service {
	case "upload-pack":
		return runGit(bin, gitDir, "upload-pack", stdin, stdout, stderr)
	case "receive-pack":
		if !s.Access.CanGitPush(user, proj) {
			return ErrAccessDenied
		}
		// SSH is interactive (server advertises refs first). Buffering stdin
		// deadlocks — unlike HTTP smart protocol, which POSTs the full body.
		// Protected-branch / secret scan / CI enqueue for SSH: post-receive hook
		// (Fase seguinte); HTTP path still uses ParseReceivePack.
		return runGit(bin, gitDir, "receive-pack", stdin, stdout, stderr)
	default:
		return fmt.Errorf("serviço não suportado")
	}
}

func runGit(bin, gitDir, subcmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	// SSH usa upload-pack/receive-pack sem --stateless-rpc (só HTTP).
	c := exec.Command(bin, "-c", "safe.directory=*", subcmd, ".")
	c.Dir = gitDir
	c.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"LC_ALL=C",
	}
	c.Stdin = stdin
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee
		}
		return err
	}
	return nil
}

func (s *Service) enforceProtectedPush(user store.User, proj store.Project, updates []forge.RefUpdate) bool {
	rules := s.Access.ProtectedBranchRules(proj.ID)
	for _, u := range updates {
		for _, rule := range rules {
			if !forge.MatchProtected([]string{rule.Pattern}, u.Ref) {
				continue
			}
			if !s.Access.CanGitPushProtected(user, proj, rule.MinPushRole) {
				return false
			}
		}
	}
	return true
}

func (s *Service) rejectSecretPush(proj store.Project, updates []forge.RefUpdate) bool {
	repo := s.Access.ProjectRepo(proj)
	reject := false
	for _, u := range updates {
		if forge.IsZeroOID(u.NewHex) {
			continue
		}
		if !forge.RevHasPrivateKey(s.Access.GitDir, repo, u.NewHex) {
			continue
		}
		reject = true
		_ = forge.ResetRef(s.Access.GitDir, repo, u.Ref, u.OldHex)
		_ = s.Access.DB.Create(&store.SecAlert{
			ProjectID: proj.ID,
			Kind:      store.SecKindSecret,
			Severity:  "critical",
			Title:     "chave privada no push",
			Tool:      "receive-pack",
			Status:    store.SecStatusOpen,
		}).Error
	}
	return reject
}

func (s *Service) enqueuePushJobs(proj store.Project, updates []forge.RefUpdate, actor string) {
	for _, u := range updates {
		if forge.IsZeroOID(u.NewHex) || !strings.HasPrefix(u.Ref, "refs/heads/") {
			continue
		}
		var last store.CiJob
		number := uint(1)
		if err := s.Access.DB.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		}
		_ = s.Access.DB.Create(&store.CiJob{
			ProjectID: proj.ID,
			Number:    number,
			Trigger:   "push",
			Ref:       u.Ref,
			SHA:       u.NewHex,
			Workflow:  "ci",
			Actor:     strings.TrimSpace(actor),
			Status:    store.CiPending,
		}).Error
	}
}
