// Package provision contém a lógica de provisionamento de contas Unix
// por usuário do painel (Fase 13 — ver PLAN.md §6.9). É usada pelo binário
// privilegiado cmd/xvpn-user-provision, que roda como root via sudoers.d
// restrito, e nunca pelo processo xvpn-server (que só chama o binário
// via os/exec — ver internal/userprovision).
//
// A lógica fica dividida em duas camadas para ser testável sem root:
//
//   - Funções puras (ValidUsername, SSHDMatchConfig, SambaShareConfig):
//     sem efeitos colaterais, testáveis diretamente.
//   - Orquestração (Create, EnableSFTP, EnableSamba, Disable): usa uma
//     interface Runner para isolar as chamadas de sistema (useradd,
//     mkdir, write, systemctl reload). Em produção usa o osRunner (que
//     chama os/exec de verdade); em teste usa um fakeRunner que registra
//     as chamadas em memória.
package provision

import "regexp"

// usernameRegex é o padrão de nome de usuário Unix aceito pelo
// provisionador (ver PLAN.md §6.9): começa com letra minúscula, 3–32
// caracteres no total, só minúsculas/dígitos/underscore/hífen. Mais
// restrito que o default do useradd, para evitar nomes que poderiam
// colidir com contas de sistema ou ter significado especial no shell
// (ex.: começar com dígito, conter espaço, etc.). Compilado uma vez no
// import do pacote — a regex é imutável.
var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

// ValidUsername reporta se name é um nome de usuário aceito pelo
// provisionador. Roda ANTES de qualquer chamada de sistema, para nunca
// repassar string livre do painel para useradd/smb.conf/sshd_config —
// mesmo que o sudoers só permita o caminho exato do binário, validar
// aqui é defesa em profundidade contra injeção de argumento.
func ValidUsername(name string) bool {
	return usernameRegex.MatchString(name)
}
