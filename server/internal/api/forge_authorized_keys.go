package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const maxForgeSSHKeysPerUser = 32

// renderForgeAuthorizedKeys monta o authorized_keys global do usuário Unix
// `git`. Cada linha força command= para o shell do forge com o username do
// painel — o sshd não concede shell interativo.
func (a *App) renderForgeAuthorizedKeys() (string, error) {
	var keys []store.ForgeSSHKey
	if err := a.Store.DB.Order("id").Find(&keys).Error; err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", nil
	}

	userIDs := make(map[uint]struct{})
	for _, k := range keys {
		userIDs[k.UserID] = struct{}{}
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	var users []store.User
	if err := a.Store.DB.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return "", err
	}
	byID := make(map[uint]store.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	shell := "/opt/xvpn/bin/xvpn-git-shell"
	if a.Config != nil && strings.TrimSpace(a.Config.GitShellPath) != "" {
		shell = strings.TrimSpace(a.Config.GitShellPath)
	}

	lines := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		u, ok := byID[k.UserID]
		if !ok || u.Username == "" {
			continue
		}
		line := strings.TrimSpace(k.PublicKey)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id := fields[0] + " " + fields[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		opts := fmt.Sprintf(`environment="LC_ALL=C LANG=C",command="%s %s",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty`, shell, u.Username)
		lines = append(lines, opts+" "+line)
	}
	return strings.Join(lines, "\n"), nil
}

func (a *App) applyForgeAuthorizedKeys(ctx context.Context) error {
	if a.UserProvisioner == nil {
		return nil
	}
	content, err := a.renderForgeAuthorizedKeys()
	if err != nil {
		return err
	}
	return a.UserProvisioner.ApplyGitSSHKeys(ctx, content)
}
