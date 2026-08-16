package provision

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidUsername é devolvido quando o username não passa no regex
// (ver ValidUsername). O binário traduz isso pra saída não-zero e
// mensagem clara; o chamador (internal/userprovision) nunca vê isso em
// produção porque o handler valida no painel antes de chamar — defesa
// em profundidade.
var ErrInvalidUsername = errors.New("username inválido (deve começar com letra minúscula, 3-32 chars, só minúsculas/dígitos/underscore/hífen)")

// ErrUsernameCollision é devolvido quando o username do painel colide
// com uma conta Unix pré-existente que NÃO foi criada pelo
// provisionador (ex.: root, www-data, sshd). Sem isso, um admin que
// criasse um usuário "root" no painel e ligasse SFTP faria o binário
// escrever um `Match User root` no sshd, sobrescrevendo o acesso SSH do
// root de verdade (Bugbot: "Rename orphans"/colisão com conta de
// sistema). A distinção "nosso" vs "alheio" é feita checando home e
// shell: contas criadas aqui têm home=/home/<username> e
// shell=/usr/sbin/nologin (ver Create). Se o usuário existe mas não
// bate com isso, recusamos em vez de operar em cima.
var ErrUsernameCollision = errors.New("username colide com conta de sistema pré-existente não criada pelo XVPN")

// homeDir devolve o caminho absoluto da raiz do chroot/home do usuário.
// Centralizado pra Create/EnableSFTP/Disable usarem o mesmo.
func homeDir(username string) string {
	return filepath.Join("/home", username)
}

// filesDir devolve o caminho da subpasta que o usuário pode escrever
// (e que também vira o share Samba). Dentro da raiz do chroot.
func filesDir(username string) string {
	return filepath.Join(homeDir(username), "files")
}

// sshDir devolve o caminho do .ssh dentro da raiz do chroot (root:root,
// onde o authorized_keys vive — sshd lê pré-chroot).
func sshDir(username string) string {
	return filepath.Join(homeDir(username), ".ssh")
}

// Create provisiona a conta Unix e a estrutura de diretórios para um
// usuário do painel (PLAN.md §6.9). Idempotente: pode ser chamado de novo
// sem efeito se a conta e os diretórios já existirem com o dono/permissão
// certos. Não habilita SFTP nem Samba — para isso use EnableSFTP/EnableSamba.
func Create(r Runner, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	home := homeDir(username)
	files := filesDir(username)
	ssh := sshDir(username)

	exists, err := r.UserExists(username)
	if err != nil {
		return err
	}
	if !exists {
		if err := r.AddSystemUser(username, home); err != nil {
			return err
		}
	} else {
		// O usuário já existe — precisa ser uma conta que NÓS criamos
		// (re-run idempotente do reconcile), não uma conta de sistema
		// alheia. Validamos pelos campos que o provisionador fixa em
		// AddSystemUser: home=/home/<username> e shell=/usr/sbin/nologin.
		// Se bater, é nosso (re-run ok); se não, é colisão com conta de
		// sistema (ex.: root, www-data) — recusa pra não sobrescrever
		// config SSH/Samba de conta alheia (Bugbot: colisão com conta
		// de sistema).
		uHome, uShell, lerr := r.LookupUser(username)
		if lerr != nil {
			return lerr
		}
		if uHome != home || uShell != "/usr/sbin/nologin" {
			return ErrUsernameCollision
		}
	}

	// Raiz do chroot: root:root 0755. O useradd --no-create-home não
	// cria, então criamos e fixamos o dono/permissão explicitamente —
	// exigência do ChrootDirectory do sshd (todo o caminho deve ser
	// root:root sem escrita por grupo/outro), senão o sshd recusa o login.
	if err := r.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("criando %s: %w", home, err)
	}
	if err := r.Chown(home, 0, 0); err != nil {
		return fmt.Errorf("chown root:root %s: %w", home, err)
	}
	if err := r.Chmod(home, 0o755); err != nil {
		return fmt.Errorf("chmod 0755 %s: %w", home, err)
	}

	uid, gid, err := r.LookupUIDGID(username)
	if err != nil {
		return err
	}

	// files/: dono do usuário, 0700. É o que o SFTP mostra como raiz
	// visível e o mesmo path que o share Samba aponta — um único dado,
	// dois protocolos.
	if err := r.MkdirAll(files, 0o700); err != nil {
		return fmt.Errorf("criando %s: %w", files, err)
	}
	if err := r.Chown(files, uid, gid); err != nil {
		return fmt.Errorf("chown %d:%d %s: %w", uid, gid, files, err)
	}
	if err := r.Chmod(files, 0o700); err != nil {
		return fmt.Errorf("chmod 0700 %s: %w", files, err)
	}
	if err := r.GrantXvpnACL(files, username); err != nil {
		return fmt.Errorf("acl xvpn em %s: %w", files, err)
	}

	// .ssh/: root:root 0755 (dentro da raiz do chroot, mas o usuário não
	// escreve aqui — só o binário privilegiado escreve o authorized_keys
	// a partir da chave pública colada no painel).
	if err := r.MkdirAll(ssh, 0o755); err != nil {
		return fmt.Errorf("criando %s: %w", ssh, err)
	}
	if err := r.Chown(ssh, 0, 0); err != nil {
		return fmt.Errorf("chown root:root %s: %w", ssh, err)
	}
	if err := r.Chmod(ssh, 0o755); err != nil {
		return fmt.Errorf("chmod 0755 %s: %w", ssh, err)
	}
	return nil
}

// EnableSFTP habilita acesso SFTP (chave pública, sem shell, chrootado)
// para o usuário. Idempotente. sshPublicKey é o conteúdo do
// authorized_keys (uma ou mais chaves, uma por linha); pode ser vazio
// para criar a estrutura do drop-in sshd deixando o acesso efetivamente
// bloqueado até o admin colar uma chave — mas o handler do painel deve
// validar e exigir uma chave antes de chamar (ver users_handler).
func EnableSFTP(r Runner, username, sshPublicKey string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	if err := Create(r, username); err != nil {
		return err
	}
	dropIn := sshdDropInPath(username)
	if err := r.WriteFile(dropIn, SSHDMatchConfig(username), 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", dropIn, err)
	}
	ak := authorizedKeysPath(username)
	content := strings.TrimRight(sshPublicKey, "\n") + "\n"
	if err := r.WriteFile(ak, content, 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", ak, err)
	}
	// authorized_keys precisa ser root:root (já é, porque .ssh é root:root
	// e WriteFile cria com o dono do processo = root), mas garantimos
	// explicitamente pra não depender do umask do WriteFile.
	if err := r.Chown(ak, 0, 0); err != nil {
		return fmt.Errorf("chown root:root %s: %w", ak, err)
	}
	if err := r.ReloadSSH(); err != nil {
		return fmt.Errorf("recarregando sshd após habilitar SFTP de %q: %w", username, err)
	}
	return nil
}

// EnableSamba habilita o share [home-<username>] apontando para
// /home/<username>/files, com guest + force user (auth Samba via VPN,
// ver SambaShareConfig). Idempotente. Não cria entrada smbpasswd (guest
// não precisa).
func EnableSamba(r Runner, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	if err := Create(r, username); err != nil {
		return err
	}
	// O diretório de includes pode não existir na primeira chamada
	// (configurado no deploy, mas garantimos aqui também — idempotente).
	if err := r.MkdirAll(sambaIncludeDir, 0o755); err != nil {
		return fmt.Errorf("criando %s: %w", sambaIncludeDir, err)
	}
	share := sambaShareIncludePath(username)
	if err := r.WriteFile(share, SambaShareConfig(username), 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", share, err)
	}
	// Reescreve o agregado xvpn-shares.conf (o `include` do Samba
	// não suporta glob — ver samba.go). Tem que ser antes do reload
	// pra o testparm validar a config final.
	if err := regenerateSambaAggregate(r); err != nil {
		return fmt.Errorf("regenerando agregado Samba: %w", err)
	}
	if err := r.ReloadSamba(); err != nil {
		return fmt.Errorf("recarregando smbd após habilitar Samba de %q: %w", username, err)
	}
	return nil
}

// Disable remove o acesso SFTP e Samba do usuário (fragmentos sshd e
// samba), recarregando os serviços só se algo foi efetivamente removido.
// Não apaga a conta Unix nem /home/<username>/files — dados são
// preservados (ver PLAN.md §6.9: "disable remove acesso de fato", não
// "apaga arquivos"). Para remover a conta inteira seria um subcomando
// à parte, fora do escopo desta fase.
func Disable(r Runner, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	removed := false
	removedSamba := false
	dropIn := sshdDropInPath(username)
	if exists, err := r.FileExists(dropIn); err != nil {
		return err
	} else if exists {
		if err := r.RemoveFile(dropIn); err != nil {
			return fmt.Errorf("removendo %s: %w", dropIn, err)
		}
		removed = true
	}
	share := sambaShareIncludePath(username)
	if exists, err := r.FileExists(share); err != nil {
		return err
	} else if exists {
		if err := r.RemoveFile(share); err != nil {
			return fmt.Errorf("removendo %s: %w", share, err)
		}
		removed = true
		removedSamba = true
	}
	if !removed {
		return nil
	}
	// Se removeu o share Samba, regenera o agregado antes do reload.
	if removedSamba {
		if err := regenerateSambaAggregate(r); err != nil {
			return fmt.Errorf("regenerando agregado Samba: %w", err)
		}
	}
	// Recarrega ambos — não vale a pena distinguir qual foi removido
	// (um reload de serviço que não mudou config é barato e inofensivo).
	if err := r.ReloadSSH(); err != nil {
		return fmt.Errorf("recarregando sshd após disable de %q: %w", username, err)
	}
	if err := r.ReloadSamba(); err != nil {
		return fmt.Errorf("recarregando smbd após disable de %q: %w", username, err)
	}
	return nil
}

// DisableSFTP remove só o acesso SFTP (fragmento sshd), mantendo o
// Samba intacto. Necessário porque os toggles SFTP/Samba são
// independentes no painel (ver ROADMAP.md Fase 13): o admin pode
// desligar SFTP e manter Samba, ou vice-versa. O Disable "ambos"
// acima é um atalho pra quando os dois voltam a false ao mesmo tempo.
func DisableSFTP(r Runner, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	dropIn := sshdDropInPath(username)
	exists, err := r.FileExists(dropIn)
	if err != nil {
		return err
	}
	if !exists {
		return nil // já estava desligado — idempotente
	}
	if err := r.RemoveFile(dropIn); err != nil {
		return fmt.Errorf("removendo %s: %w", dropIn, err)
	}
	if err := r.ReloadSSH(); err != nil {
		return fmt.Errorf("recarregando sshd após disable-sftp de %q: %w", username, err)
	}
	return nil
}

// DisableSamba remove só o share Samba, mantendo o SFTP intacto. Mesma
// motivação de DisableSFTP — toggles independentes no painel.
func DisableSamba(r Runner, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	share := sambaShareIncludePath(username)
	exists, err := r.FileExists(share)
	if err != nil {
		return err
	}
	if !exists {
		return nil // já estava desligado — idempotente
	}
	if err := r.RemoveFile(share); err != nil {
		return fmt.Errorf("removendo %s: %w", share, err)
	}
	if err := regenerateSambaAggregate(r); err != nil {
		return fmt.Errorf("regenerando agregado Samba: %w", err)
	}
	if err := r.ReloadSamba(); err != nil {
		return fmt.Errorf("recarregando smbd após disable-samba de %q: %w", username, err)
	}
	return nil
}

// EnsureUnused é uma checagem defensiva que o handler pode chamar antes
// de provisionar: garante que o username não colide com uma conta de
// sistema pré-existente que o provisionador não deveria tocar. Como o
// regex já proíbe nomes que poderiam confundir (começar com dígito, etc.)
// e o useradd recusaria duplicatas, isto é defesa em profundidade —
// exposto publicamente porque o handler do painel pode querer rejeitar
// o username antes mesmo de chamar o binário privilegiado.
func EnsureUnused(r Runner, username string) error {
	exists, err := r.UserExists(username)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("usuário %q já existe no sistema", username)
	}
	return nil
}

// os.FileMode re-exportado pra conveniência dos testes que montam um
// fakeRunner — evita import redundante nos arquivos de teste.
var _ os.FileMode = 0o644
