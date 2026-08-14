# Política de Segurança — XVPN

Este documento descreve o modelo de ameaças, as garantias de segurança do design e o que fazer em caso de suspeita de comprometimento. Complementa as decisões justificadas em [`PLAN.md`](./PLAN.md) (seção 3 e 9).

## Repositório público — o que isso muda (e o que não muda)

O repositório do XVPN é público no GitHub (decisão para viabilizar *branch protection* real na `main` — indisponível de graça para repositórios privados em conta pessoal). Isso significa:

- IP do VPS, domínios e decisões de arquitetura em `PLAN.md`/`README.md` são visíveis publicamente. Isso **não é considerado segredo** neste projeto — é informação sobre a topologia, não uma credencial. Conhecer o IP de um servidor não dá acesso a nada por si só, se o hardening (SSH por chave, firewall, Samba/FileBrowser restritos à VPN) estiver correto.
- O que **continua** proibido de aparecer no repositório, público ou não: chaves privadas WireGuard/SSH, tokens de convite/JWT reais, senhas, o banco `xvpn.db`. Isso é garantido por `.gitignore` + `.githooks/pre-commit`, não pela visibilidade do repositório.
- Se em algum momento o cálculo de risco mudar (ex.: o projeto crescer e a exposição do IP/domínio real virar um problema), a alternativa é remover as referências ao IP/domínio real da documentação pública e mantê-las só em um arquivo local não versionado — não voltar a deixar o repositório privado só para recuperar a *branch protection* seria um passo atrás.

## Modelo de ameaças (resumo)

O XVPN assume:

- O VPS (`206.189.224.72`) é o único ponto central de confiança da rede privada. Comprometê-lo compromete a VPN inteira — não há redundância/alta disponibilidade neste projeto (uso pessoal, ver `PLAN.md` §10).
- Clientes desktop podem ser perdidos/roubados — por isso, a revogação de um dispositivo deve ser possível a qualquer momento pelo painel, com efeito imediato (sem esperar expiração de nada).
- O tráfego entre cliente e servidor passa pela internet pública e é protegido apenas pela criptografia do WireGuard (Curve25519/ChaCha20) — não há camada adicional de TLS no túnel em si (não é necessária, é redundante com o próprio WireGuard).
- O compartilhamento de arquivos (Samba/FileBrowser) é considerado **dado sensível** e nunca deve ser alcançável fora do túnel VPN.

## Garantias de design (o que já está arquiteturalmente resolvido)

| Garantia | Como é implementada |
|---|---|
| Chave privada nunca sai do dispositivo do cliente | Gerada localmente pelo helper privilegiado; servidor só recebe/armazena chave pública ([`PLAN.md` §3.5](./PLAN.md#35-geração-de-chaves-onde-nasce-a-chave-privada)) |
| Revogação imediata de dispositivo | `DELETE /api/devices/:id` remove o peer da interface `wg0` via `wgctrl` em tempo real, sem reiniciar a interface |
| Compartilhamento de arquivos nunca exposto à internet | Samba com `bind interfaces only = yes` / `interfaces = wg0 lo`; FileBrowser escutando só em `10.66.66.1` — nunca em `0.0.0.0` nem cadastrado em domínio público/Nginx |
| Superfície de ataque mínima do painel | Painel/API atrás de Nginx com TLS (Let's Encrypt), autenticação JWT, hash de senha Argon2id |
| Firewall padrão-nega | `ufw` com política `deny incoming`, liberando só `22`, `80`, `443`, `51820/udp` |

## O que NÃO fazer (violação das garantias acima)

- Nunca fazer bind de Samba, FileBrowser, ou qualquer serviço interno em `0.0.0.0` — sempre bind explícito em `10.66.66.1` (ou `127.0.0.1` para serviços atrás do Nginx).
- Nunca gerar ou transmitir uma chave privada WireGuard do servidor para um cliente.
- Nunca desabilitar o `ufw` ou fazer flush de regras (`ufw disable`, `iptables -F`) em produção sem um plano de rollback imediato — há um hook do Cursor que bloqueia isso por padrão.
- Nunca commitar `xvpn.db`, arquivos `.key`, `.conf` de WireGuard com chaves reais, ou tokens de convite no Git.

## Hardening aplicado/planejado no servidor

Ver checklist completo em [`ROADMAP.md` — Fase 0](./ROADMAP.md#fase-0--hardening-e-provisionamento-base-do-vps). Pontos-chave:

- SSH: apenas autenticação por chave (`PasswordAuthentication no`), `PermitRootLogin prohibit-password`.
- `fail2ban` para proteção contra força bruta em SSH.
- `unattended-upgrades` para patches de segurança automáticos do SO.
- Backups regulares do banco de dados (`xvpn.db`) com rotação.

### Binário privilegiado `xvpn-user-provision` (Fase 13)

O painel (`xvpn-server`, processo não-root) precisa criar contas Unix e
editar `sshd_config.d`/`smb.conf.d` para provisionar SFTP/Samba por
usuário (ver [`PLAN.md` §6.9](./PLAN.md#69-contas-unix-reais-por-usuário-sftp--samba-integrados)).
Em vez de rodar o servidor inteiro como root, há um binário mínimo e
auditável (`server/cmd/xvpn-user-provision`) que faz só essas
operações, invocado via `sudo -n` com escopo restrito:

- **`/etc/sudoers.d/xvpn-user-provision`** (permissão `0440`):
  ```
  xvpn ALL=(root) NOPASSWD: /opt/xvpn/bin/xvpn-user-provision
  ```
  Sem wildcard de argumento — o `sudo` só aceita o caminho exato do
  binário, sem argumentos. Os subcomandos (`create`, `enable-sftp`,
  `enable-samba`, `disable`, `disable-sftp`, `disable-samba`) são
  parseados pelo próprio binário, que valida o username com regex
  `^[a-z][a-z0-9_-]{2,31}$` antes de qualquer syscall e lê a chave
  pública SSH do stdin (não de argumento — evita vazar no `ps`/`/proc`).
- **Validação de config antes de reload**: o binário roda `sshd -t` e
  `testparm -s` antes de recarregar os serviços; se a config gerada for
  inválida, o reload não acontece e o binário devolve erro.
- **Defesa em profundidade**: SFTP e Samba escutam só em `wg0`
  (`10.66.66.1`) — nunca em `0.0.0.0`/`etho`. Mesmo que o `sudoers.d`
  fosse comprometido, o atacante não expõe os serviços na internet.
- **Auditoria**: cada enable/disable é logado no audit log do painel
  (`user.file_access`, actor = admin que clicou o toggle), não pelo
  binário. O binário só loga erros no stderr.

Use a skill `vps-security-audit` (`.cursor/skills/vps-security-audit/`) para revalidar esses pontos periodicamente — ela roda os mesmos checks read-only usados no diagnóstico inicial do projeto.

### Isolamento cross-user no Samba (Fase 13)

**Decisão registrada:** os shares Samba per-user (`[home-<username>]`) usam `guest ok = yes` + `force user = <username>` **sem** `valid users`. Consequência: **qualquer peer autenticado na VPN pode acessar o share de qualquer usuário se souber o nome** — não há isolamento entre usuários *dentro* da VPN. A VPN é tratada como domínio de confiança única.

**Por que aceitamos isso:** reintroduzir `valid users` exigiria uma senha Samba por usuário (gerada/armazenada/rotacionada pelo painel), reabrindo a superfície de credencial que a Fase 5 descartou. A troca foi **simplicidade > isolamento granular**, aceita em revisão de segurança da Fase 13 (Bugbot sinalizou como HIGH; mitigação escolhida: aceitar e documentar).

**O que isso NÃO quebra:**
- O share `[shared]` (comum) **passa a usar o mesmo modelo guest** na Fase 14 (`guest ok = yes` + `force user = xvpn-shared` + `force group = xvpn-samba`): qualquer peer autenticado na VPN alcança `/srv/xvpn/shared` sem senha Samba — alinhado à decisão “VPN como barreira” dos shares pessoais. Contas `smbpasswd` manuais deixam de ser o caminho normal (skill `samba-user-ops` só cobre legado/`xvpn-shared`).
- SFTP **não** é afetado — usa chave pública por usuário (e, na Fase 14.2, união com chaves auto-registradas por dispositivo), isolamento natural por credencial.

**Mitigações em vigor:**
- Samba escuta só em `wg0` (`10.66.66.1`) — nunca na internet. O ataque só é viável de dentro da VPN.
- Shares são `browseable = yes` (qualquer peer vê a lista de `home-*` via `smbclient -L`), então descobrir shares = descobrir usernames do painel. Isso é fraco como defesa, mas o username já é necessário pra qualquer acesso (SFTP/SSH), então não é informação nova sensível.

**Se a ameaça voltar a ser inaceitável:** reintroduzir `valid users = <username>` + senha Samba por usuário (reabre a superfície de credencial descartada na Fase 5). *Não implementado hoje.*

## Rotação e revogação de chaves

- **Dispositivo perdido/comprometido**: revogar imediatamente pelo painel (remove o peer do `wg0`). Não é necessário rotacionar a chave do servidor nesse caso — só a chave pública daquele dispositivo específico é invalidada.
- **Suspeita de comprometimento do servidor**: rotacionar a chave privada do servidor (`wg genkey`), o que invalida *todos* os peers de uma vez — todos os dispositivos precisarão reenrolar. Nesse cenário, também revisar `xvpn.db` em busca de usuários/dispositivos não reconhecidos antes de reconectar qualquer cliente.

## Resposta a incidentes (procedimento de emergência)

Se houver suspeita real de comprometimento do VPS:

1. Isolar: `ufw deny 51820/udp` temporariamente (derruba a VPN, mas contém o problema) e revisar `ss -tulnp` / `who` / `last` no servidor.
2. Rotacionar credenciais: nova chave WireGuard do servidor, novo JWT secret, revisar/trocar a chave SSH se houver qualquer dúvida sobre como o acesso foi obtido.
3. Auditar: revisar logs de auditoria do painel (`AuditLog`), logs do `sshd`, e o histórico de comandos relevantes.
4. Reconstruir se necessário: o provisionamento (Fase 0/1 do `ROADMAP.md`) foi desenhado para ser reproduzível — preferir reprovisionar um VPS novo a "limpar" um comprometido, se a suspeita for séria.
5. Reenrolar dispositivos legítimos manualmente, um a um, validando a identidade de quem está pedindo o convite.

## Reportando um problema de segurança

Projeto pessoal/privado no momento — reportar diretamente ao mantenedor. Se/quando o projeto se tornar público, este documento será atualizado com um canal formal (e-mail dedicado ou `SECURITY.md` do GitHub com política de divulgação responsável).
