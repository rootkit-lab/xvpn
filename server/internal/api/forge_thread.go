package api

import (
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"gorm.io/gorm"
)

func (a *App) createProjectThread(tx *gorm.DB, proj store.Project, user store.User, kind, title, firstMsg, postBody string) (threadID uint, postID uint, err error) {
	th := store.DirectThread{Kind: kind, Title: title}
	if err := tx.Create(&th).Error; err != nil {
		return 0, 0, err
	}
	seen := map[uint]struct{}{user.ID: {}}
	if err := tx.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: user.ID}).Error; err != nil {
		return 0, 0, err
	}
	var members []store.ProjectMember
	if err := tx.Where("project_id = ?", proj.ID).Find(&members).Error; err != nil {
		return 0, 0, err
	}
	for _, m := range members {
		if _, ok := seen[m.UserID]; ok {
			continue
		}
		seen[m.UserID] = struct{}{}
		if err := tx.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: m.UserID}).Error; err != nil {
			return 0, 0, err
		}
	}
	msg := store.Message{ThreadKind: store.ThreadKindDM, ThreadID: th.ID, AuthorID: user.ID, Kind: "text", Body: firstMsg}
	if err := tx.Create(&msg).Error; err != nil {
		return 0, 0, err
	}
	slug := proj.Slug
	post := store.SocialPost{AuthorID: user.ID, Body: truncateRunes(postBody, maxPostRunes), Kind: "text", ProjectSlug: &slug}
	if err := tx.Create(&post).Error; err != nil {
		return 0, 0, err
	}
	return th.ID, post.ID, nil
}

func (a *App) canReporterWrite(user store.User, proj store.Project) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	return ok && role.Rank() >= store.ProjectRoleReporter.Rank()
}

func (a *App) canMaintainerWrite(user store.User, proj store.Project) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	return ok && role.Rank() >= store.ProjectRoleMaintainer.Rank()
}

func (a *App) canPushBranch(user store.User, proj store.Project, branch string) bool {
	if !a.canGitPush(user, proj) {
		return false
	}
	ref := "refs/heads/" + branch
	for _, rule := range a.protectedBranchRules(proj.ID) {
		if !forge.MatchProtected([]string{rule.Pattern}, ref) {
			continue
		}
		return a.canGitPushProtected(user, proj, rule.MinPushRole)
	}
	return true
}
