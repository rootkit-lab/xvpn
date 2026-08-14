package provision

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// sambaIncludeDir é o diretório de includes do smb.conf. O smb.conf
// principal precisa ter uma linha `include = /etc/samba/smb.conf.d/
// xvpn-shares.conf` para que os shares por usuário sejam carregados
// — isso é configurado no deploy da Fase 13 (ver ROADMAP.md), não
// pelo binário.
//
// Nota: o `include` do Samba NÃO suporta glob (*.conf) como o Include
// do sshd — ele inclui um único arquivo. Por isso o binário mantém um
// agregado (xvpn-shares.conf) que lista um `include =` por share
// per-user; o smb.conf inclui só esse agregado.
//
// E, também diferente do sshd, o `include` do Samba não abre escopo
// próprio: a seção corrente atravessa a fronteira do include. Como o
// agregado declara seções [home-<user>], essa linha `include` tem que
// ser a ÚLTIMA do smb.conf — dentro do [global] ela transforma todo
// parâmetro seguinte em parâmetro de share e derruba o bind exclusivo
// em wg0. CheckSambaTestparmOutput existe pra barrar esse estado antes
// de qualquer reload.
const sambaIncludeDir = "/etc/samba/smb.conf.d"

// sambaAggregatePath é o arquivo único que o smb.conf inclui. Contém
// uma linha `include = <caminho>` para cada share per-user presente
// no diretório. Regenerado a cada enable/disable (ver
// regenerateSambaAggregate).
const sambaAggregatePath = "/etc/samba/smb.conf.d/xvpn-shares.conf"

// sambaShareIncludePath devolve o caminho do fragmento de smb.conf que
// define o share [home-<username>] apontando para /home/<username>/files.
// Um arquivo por usuário, prefixado "xvpn-home-" para identificação e limpeza.
func sambaShareIncludePath(username string) string {
	return fmt.Sprintf("%s/xvpn-home-%s.conf", sambaIncludeDir, username)
}

// regenerateSambaAggregate reescreve xvpn-shares.conf listando um
// `include =` para cada fragmento xvpn-home-*.conf presente no
// diretório. É a ponte entre os arquivos per-user (fonte da verdade,
// fáceis de adicionar/remover) e o `include` único do Samba (que não
// suporta glob). Idempotente: chamada a cada enable/disable de Samba.
//
// Ordem alfabética pra o arquivo ser determinístico (diffs limpos,
// testes reproduzíveis). Exclui o próprio agregado e qualquer arquivo
// que não case xvpn-home-*.conf.
func regenerateSambaAggregate(r Runner) error {
	names, err := r.ReadDir(sambaIncludeDir)
	if err != nil {
		return fmt.Errorf("lendo %s: %w", sambaIncludeDir, err)
	}
	var shares []string
	for _, n := range names {
		if strings.HasPrefix(n, "xvpn-home-") && strings.HasSuffix(n, ".conf") {
			shares = append(shares, n)
		}
	}
	sort.Strings(shares)
	var b strings.Builder
	b.WriteString("# Gerado por xvpn-user-provision (Fase 13, PLAN.md §6.9).\n")
	b.WriteString("# NÃO editar — regenerado a cada toggle de Samba no painel.\n")
	for _, n := range shares {
		fmt.Fprintf(&b, "include = %s/%s\n", sambaIncludeDir, n)
	}
	if err := r.WriteFile(sambaAggregatePath, b.String(), 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", sambaAggregatePath, err)
	}
	return nil
}

// sambaSectionLeakMarker é o trecho do aviso que o testparm imprime
// quando lê um parâmetro que só faz sentido em [global] dentro de uma
// seção de serviço: "Global parameter interfaces found in service
// section!".
const sambaSectionLeakMarker = "found in service section"

// sambaWG0Net / sambaLoopbackNet são as únicas redes nas quais o Samba
// pode bindar (invariante 2 do AGENTS.md). Qualquer outro endereço —
// IP público do VPS, eth0, 0.0.0.0 — é rejeitado.
var (
	sambaWG0Net      = mustCIDR("10.66.66.0/24")
	sambaLoopbackNet = mustCIDR("127.0.0.0/8")
)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("provision: CIDR inválido: " + s)
	}
	return n
}

// sambaGlobalChecks são as asserções sobre o [global] efetivo que
// protegem o invariante 2 do AGENTS.md (Samba nunca escuta fora da wg0).
// Só diretivas de confinamento de rede entram aqui — não é um espelho do
// smb.conf inteiro. "interfaces" tem validador próprio (ver
// sambaInterfacesOK): substring não basta, porque "10.66.66.1 eth0" ou
// "10.66.66.1 206.189.224.72" passariam e ainda assim abriria bind fora
// da wg0.
var sambaGlobalChecks = []struct {
	name   string // nome do parâmetro como o testparm -s imprime
	equals string // valor exato exigido (case-insensitive); "" = não checa
}{
	{name: "bind interfaces only", equals: "Yes"},
	{name: "interfaces"}, // validado por sambaInterfacesOK
}

// CheckSambaTestparmOutput reprova a saída do testparm quando ela indica
// que a config efetiva perdeu o confinamento de rede do Samba. Função
// pura pra ser testável sem root e sem testparm instalado.
//
// Existe porque o testparm sai com código 0 e imprime "Loaded services
// file OK." mesmo quando o [global] foi esvaziado: o problema aparece só
// como aviso no stderr. A causa conhecida é a linha `include` dos shares
// por usuário estar antes do fim do [global] no smb.conf — o `include`
// do Samba insere o arquivo literalmente e a seção corrente atravessa a
// fronteira, então tudo que vem depois dele passa a pertencer ao
// primeiro share [home-<user>]. O efeito é "bind interfaces only" voltar
// ao default "no" e o smbd escutar em TODAS as interfaces, violando o
// invariante 2 do AGENTS.md. Ver o comentário no fim de
// server/deploy/samba/smb.conf.
func CheckSambaTestparmOutput(out string) error {
	if leaked := sambaLeakedGlobals(out); len(leaked) > 0 {
		return fmt.Errorf("testparm aceitou a config, mas %d parâmetro(s) de [global] "+
			"vazaram para uma seção de serviço (%s). Causa provável: a linha `include` "+
			"dos shares por usuário não é a última de /etc/samba/smb.conf — dentro do "+
			"[global] ela faz todo parâmetro seguinte pertencer ao primeiro share "+
			"[home-<user>], o smbd volta a escutar em todas as interfaces e o guest dos "+
			"shares para de funcionar. Corrija movendo o `include` para o fim do arquivo "+
			"(ver server/deploy/samba/smb.conf). Reload abortado.\n\nSaída do testparm:\n%s",
			len(leaked), strings.Join(leaked, ", "), out)
	}
	if missing := sambaMissingGlobals(out); len(missing) > 0 {
		return fmt.Errorf("testparm aceitou a config, mas o [global] efetivo não tem o "+
			"confinamento de rede esperado (%s). O Samba só pode escutar em wg0 "+
			"(10.66.66.1) — ver invariante 2 do AGENTS.md. Confira /etc/samba/smb.conf "+
			"contra server/deploy/samba/smb.conf. Reload abortado.\n\nSaída do testparm:\n%s",
			strings.Join(missing, "; "), out)
	}
	return nil
}

// sambaLeakedGlobals devolve os nomes dos parâmetros globais que o
// testparm reportou dentro de uma seção de serviço. Cai para a linha
// inteira se o formato do aviso não for o esperado, pra nunca engolir um
// aviso que não soubemos parsear.
func sambaLeakedGlobals(out string) []string {
	const prefix = "Global parameter "
	var leaked []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, sambaSectionLeakMarker) {
			continue
		}
		name := line
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			if before, _, found := strings.Cut(rest, sambaSectionLeakMarker); found {
				name = strings.TrimSpace(before)
			}
		}
		leaked = append(leaked, name)
	}
	return leaked
}

// sambaMissingGlobals confere sambaGlobalChecks contra a seção [global]
// da saída do `testparm -s`. Um parâmetro ausente dessa seção significa
// que ele está no default (o -s só imprime o que difere do default) ou
// vazou para um share — nos dois casos o confinamento caiu.
func sambaMissingGlobals(out string) []string {
	global, ok := sambaGlobalSection(out)
	if !ok {
		return []string{"a saída do testparm não tem uma seção [global]"}
	}
	var problems []string
	for _, c := range sambaGlobalChecks {
		value, present := global[c.name]
		switch {
		case !present:
			problems = append(problems, fmt.Sprintf("%q ausente do [global]", c.name))
		case value == "":
			problems = append(problems, fmt.Sprintf("%q vazio no [global]", c.name))
		case c.name == "interfaces":
			if err := sambaInterfacesOK(value); err != nil {
				problems = append(problems, err.Error())
			}
		case c.equals != "" && !strings.EqualFold(value, c.equals):
			problems = append(problems, fmt.Sprintf("%q = %q, esperado %q", c.name, value, c.equals))
		}
	}
	return problems
}

// sambaInterfacesOK exige que CADA token de "interfaces" seja um IP ou
// CIDR dentro de 10.66.66.0/24 ou 127.0.0.0/8, e que a wg0 esteja
// presente. Nomes de interface (eth0, ens3, wg0) e IPs públicos são
// rejeitados — o deploy usa CIDR por IP de propósito (ver comentário
// em server/deploy/samba/smb.conf).
func sambaInterfacesOK(value string) error {
	tokens := strings.Fields(value)
	if len(tokens) == 0 {
		return fmt.Errorf("%q vazio no [global]", "interfaces")
	}
	hasWG0 := false
	for _, tok := range tokens {
		ip, err := parseInterfaceToken(tok)
		if err != nil {
			return fmt.Errorf("%q = %q: token %q inválido (%v); só CIDR/IP em 10.66.66.0/24 ou 127.0.0.0/8",
				"interfaces", value, tok, err)
		}
		inWG0 := sambaWG0Net.Contains(ip)
		inLoop := sambaLoopbackNet.Contains(ip)
		if !inWG0 && !inLoop {
			return fmt.Errorf("%q = %q: %q (%s) fora de wg0/loopback — viola invariante 2",
				"interfaces", value, tok, ip)
		}
		if inWG0 {
			hasWG0 = true
		}
	}
	if !hasWG0 {
		return fmt.Errorf("%q = %q: falta endereço em 10.66.66.0/24 (wg0)", "interfaces", value)
	}
	return nil
}

// parseInterfaceToken aceita "10.66.66.1", "10.66.66.1/24" ou IPv6
// equivalente. Recusa nomes de interface e literais ambíguos.
func parseInterfaceToken(tok string) (net.IP, error) {
	if strings.Contains(tok, "/") {
		ip, _, err := net.ParseCIDR(tok)
		if err != nil {
			return nil, err
		}
		return ip, nil
	}
	ip := net.ParseIP(tok)
	if ip == nil {
		return nil, fmt.Errorf("não é IP nem CIDR (nomes de interface como eth0/wg0 não são aceitos)")
	}
	return ip, nil
}

// sambaGlobalSection extrai os pares chave/valor da seção [global] da
// saída do `testparm -s`. O formato é estável nos Samba 4.x: um cabeçalho
// "[global]" e depois linhas "\tchave = valor" até o próximo "[secao]".
// O bool devolvido distingue "[global] existe mas está vazio" de "não há
// [global] nenhum" — o segundo caso indica saída inesperada, não config
// no default.
func sambaGlobalSection(out string) (map[string]string, bool) {
	params := map[string]string{}
	inGlobal, sawGlobal := false, false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inGlobal = trimmed == "[global]"
			sawGlobal = sawGlobal || inGlobal
			continue
		}
		if !inGlobal || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, found := strings.Cut(trimmed, "=")
		if !found {
			continue
		}
		params[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return params, sawGlobal
}

// SambaShareConfig devolve o conteúdo do fragmento de smb.conf que
// define o share [home-<username>]. Função pura, testável sem root.
//
// Autenticação Samba (decisão Fase 13, ver ROADMAP.md): guest + force
// user, confiando no VPN (wg0) como barreira de auth — o share só é
// alcançável via 10.66.66.1 (bind exclusivo wg0, ver Fase 5), e a
// própria VPN já é autenticada (enrollment de dispositivo). Sem senha
// Samba = sem novo segredo para o usuário decorar/relay. force user
// garante que os arquivos sejam gravados com o dono Unix correto
// (<username>), mesmo vindo de uma conexão guest.
func SambaShareConfig(username string) string {
	return fmt.Sprintf(`# Gerado por xvpn-user-provision (Fase 13, PLAN.md §6.9).
# NÃO editar manualmente — use o painel para ligar/desligar o acesso.
[home-%s]
    path = /home/%s/files
    browseable = yes
    guest ok = yes
    force user = %s
    writable = yes
`, username, username, username)
}
