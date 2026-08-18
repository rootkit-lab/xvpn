package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/provision"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const codespaceIdle = 30 * time.Minute

var codespaceHostRe = regexp.MustCompile(`^cs-([a-f0-9]{12})\.corp\.(ihuull\.com|localhost)$`)

func codespaceRuntimeHost(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	m := codespaceHostRe.FindStringSubmatch(strings.ToLower(h))
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func codespaceRuntimeURL(id string) string {
	return "https://cs-" + id + ".corp.ihuull.com"
}

func codespaceLoginURL(returnTo string) string {
	u, err := url.Parse("https://xauth.ihuull.com/login")
	if err != nil {
		return "https://xauth.ihuull.com/login"
	}
	if safe := auth.SafeReturnURL(returnTo); safe != "" {
		q := u.Query()
		q.Set("return", safe)
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func hashCodespaceToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func newCodespaceToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (a *App) maybeCodespaceProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := codespaceRuntimeHost(c.Request.Host)
		if id == "" {
			c.Next()
			return
		}
		if isCodespaceGinPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		a.serveCodespaceRuntime(c, id)
		c.Abort()
	}
}

func (a *App) serveCodespaceRuntime(c *gin.Context, id string) {
	token := auth.TokenFromRequest(c)
	if token == "" {
		c.Redirect(http.StatusFound, codespaceLoginURL(codespaceRuntimeURL(id)+c.Request.URL.RequestURI()))
		return
	}
	claims, err := a.Tokens.Parse(token)
	if err != nil {
		c.Redirect(http.StatusFound, codespaceLoginURL(codespaceRuntimeURL(id)))
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, claims.UserID).Error; err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var cs store.CodeSpace
	if err := a.Store.DB.Where("public_id = ?", id).First(&cs).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if cs.UserID != user.ID && !store.HasProduct(user.Role, user.Products, store.ProductForge) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if cs.Kind != store.CodespaceKindRemote || cs.Status != store.CodespaceRunning || cs.HostPort < provision.CodespacePortMin {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "codespace parado"})
		return
	}
	now := time.Now()
	_ = a.Store.DB.Model(&cs).Update("last_active_at", now).Error
	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cs.HostPort))
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	base := proxy.Director
	connTok := a.codespaceConnectionToken(id)
	proxy.Director = func(req *http.Request) {
		base(req)
		prepareCodespaceUpstream(req, connTok)
	}
	proxy.ModifyResponse = filterCodespaceSetCookie
	proxy.ServeHTTP(c.Writer, c.Request)
}

const vscodeTokenCookie = "vscode-tkn"

// prepareCodespaceUpstream tira a sessão ihuull do IDE e autentica o
// openvscode via cookie vscode-tkn. Injetar ?tkn= em todo request faz o
// servidor 302 para / (para esconder o token) e o proxy recolocava o
// parâmetro — ERR_TOO_MANY_REDIRECTS.
func prepareCodespaceUpstream(req *http.Request, connTok string) {
	req.Header.Del("Authorization")
	keep := make([]string, 0, 4)
	hasVSCodeTkn := false
	for _, ck := range req.Cookies() {
		if ck.Name == auth.SessionCookieName {
			continue
		}
		if ck.Name == vscodeTokenCookie {
			hasVSCodeTkn = true
			if connTok != "" {
				ck.Value = connTok
			}
		}
		keep = append(keep, ck.Name+"="+ck.Value)
	}
	if connTok != "" && !hasVSCodeTkn {
		keep = append(keep, vscodeTokenCookie+"="+connTok)
	}
	if len(keep) == 0 {
		req.Header.Del("Cookie")
	} else {
		req.Header.Set("Cookie", strings.Join(keep, "; "))
	}
	if connTok != "" {
		req.Header.Set("X-Connection-Token", connTok)
	}
	q := req.URL.Query()
	if q.Get("tkn") != "" {
		q.Del("tkn")
		req.URL.RawQuery = q.Encode()
	}
}

func filterCodespaceSetCookie(resp *http.Response) error {
	if resp == nil {
		return nil
	}
	cookies := resp.Cookies()
	resp.Header.Del("Set-Cookie")
	for _, ck := range cookies {
		if ck.Name == auth.SessionCookieName || !strings.HasPrefix(ck.Name, "vscode") {
			continue
		}
		resp.Header.Add("Set-Cookie", ck.String())
	}
	return nil
}

func (a *App) codespaceConnectionToken(id string) string {
	if a == nil || a.Tokens == nil {
		return ""
	}
	sum := a.Tokens.HMACHex("cs-conn:" + id)
	if len(sum) < 32 {
		return ""
	}
	return sum[:32]
}

func (a *App) allocateCodespacePort() (int, error) {
	var used []int
	_ = a.Store.DB.Model(&store.CodeSpace{}).Where("status = ? AND host_port > 0", store.CodespaceRunning).Pluck("host_port", &used)
	taken := map[int]struct{}{}
	for _, p := range used {
		taken[p] = struct{}{}
	}
	for p := provision.CodespacePortMin; p <= provision.CodespacePortMax; p++ {
		if _, ok := taken[p]; !ok {
			return p, nil
		}
	}
	return 0, fmt.Errorf("sem porta livre")
}

func (a *App) runningCodespaceCount() int64 {
	var n int64
	_ = a.Store.DB.Model(&store.CodeSpace{}).Where("status = ?", store.CodespaceRunning).Count(&n)
	return n
}

func (a *App) maybeIdleStop(cs *store.CodeSpace) {
	if cs.Kind != store.CodespaceKindRemote || cs.Status != store.CodespaceRunning {
		return
	}
	if cs.LastActiveAt == nil || time.Since(*cs.LastActiveAt) < codespaceIdle {
		return
	}
	_ = a.applyCodespace(context.Background(), cs, "stop", 0, "", "")
	cs.Status = store.CodespaceStopped
	cs.HostPort = 0
	cs.GitTokenHash = ""
	_ = a.Store.DB.Model(cs).Updates(map[string]any{"status": cs.Status, "host_port": 0, "git_token_hash": ""}).Error
}

func (a *App) remoteWorkspace(cs store.CodeSpace) string {
	return filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath), "workspace")
}

func (a *App) applyCodespace(ctx context.Context, cs *store.CodeSpace, action string, port int, token, cloneURL string) error {
	if a.UserProvisioner == nil {
		return errCodespaceProvision
	}
	bare := ""
	branch := ""
	if action == "create" || (action == "start" && token != "") {
		var proj store.Project
		if err := a.Store.DB.First(&proj, cs.ProjectID).Error; err != nil {
			return err
		}
		if action == "create" {
			p, err := forge.RepoPath(a.gitDir(), proj.Slug)
			if err != nil {
				return err
			}
			bare = p
			branch = cs.Branch
		}
		if cloneURL == "" {
			cloneURL = gitCloneHost + "/" + proj.Slug
		}
	}
	spec := provision.CsSpec{
		Action:          action,
		ID:              cs.PublicID,
		Workspace:       a.remoteWorkspace(*cs),
		BarePath:        bare,
		Branch:          branch,
		Image:           cs.Image,
		Port:            uint16(port),
		CloneURL:        cloneURL,
		GitUser:         "codespace-" + cs.PublicID,
		GitToken:        token,
		ConnectionToken: a.codespaceConnectionToken(cs.PublicID),
	}
	if spec.Image == "" {
		spec.Image = provision.DefaultCodespaceImage
	}
	if action == "create" {
		spec.Env = a.codespaceRuntimeEnvs(cs.ProjectID)
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	return a.UserProvisioner.ApplyCodespace(ctx, string(raw))
}

var errCodespaceProvision = fmt.Errorf("provisionador de codespace indisponível")
