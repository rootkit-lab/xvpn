package userprovision

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeExecutor registra cada chamada (name, args, stdin) num slice pra
// o teste poder afirmar exatamente o que foi invocado. Pode ser
// programado pra devolver um erro específico (errOn) pra testar
// tradução de erros (ExitError, ErrBinaryMissing, etc.).
type fakeExecutor struct {
	calls  []fakeCall
	errOn  string // substring de args que dispara errTo
	errTo  error
	stdout string
}

type fakeCall struct {
	name  string
	args  []string
	stdin string
}

func (f *fakeExecutor) exec(_ context.Context, name string, args []string, stdin string) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{name: name, args: append([]string(nil), args...), stdin: stdin})
	if f.errOn != "" {
		// errOn casa contra o nome do comando (ex.: "sudo") ou
		// qualquer argumento (ex.: "enable-sftp"). Pra o teste
		// simular "sudo não encontrado", errOn="sudo" casa o nome.
		if strings.Contains(name, f.errOn) {
			return []byte(f.stdout), f.errTo
		}
		for _, a := range args {
			if strings.Contains(a, f.errOn) {
				return []byte(f.stdout), f.errTo
			}
		}
	}
	return []byte(f.stdout), nil
}

func TestCreate_InvokesSudoWithRightArgs(t *testing.T) {
	fe := &fakeExecutor{}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	if err := c.Create(context.Background(), "alice"); err != nil {
		t.Fatalf("Create falhou: %v", err)
	}
	if len(fe.calls) != 1 {
		t.Fatalf("esperava 1 chamada, obtido %d: %+v", len(fe.calls), fe.calls)
	}
	call := fe.calls[0]
	if call.name != "sudo" {
		t.Errorf("esperava sudo, obtido %q", call.name)
	}
	// sudo -n <binary> create <username> — exatamente esta ordem,
	// sem sh -c, sem concatenação de string.
	wantArgs := []string{"-n", "/opt/xvpn/bin/xvpn-user-provision", "create", "alice"}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("args: esperado %v, obtido %v", wantArgs, call.args)
	}
	for i, w := range wantArgs {
		if call.args[i] != w {
			t.Errorf("args[%d]: esperado %q, obtido %q", i, w, call.args[i])
		}
	}
	if call.stdin != "" {
		t.Errorf("Create não deveria enviar stdin, obtido %q", call.stdin)
	}
}

func TestEnableSFTP_PipesKeyToStdin(t *testing.T) {
	fe := &fakeExecutor{}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	key := "ssh-ed25519 AAAA alice@host"
	if err := c.EnableSFTP(context.Background(), "alice", key); err != nil {
		t.Fatalf("EnableSFTP falhou: %v", err)
	}
	if len(fe.calls) != 1 || fe.calls[0].args[2] != "enable-sftp" {
		t.Fatalf("esperava 1 chamada enable-sftp: %+v", fe.calls)
	}
	if fe.calls[0].stdin != key {
		t.Errorf("stdin deveria ser a chave, obtido %q", fe.calls[0].stdin)
	}
}

func TestEnableSamba_Dispatch(t *testing.T) {
	fe := &fakeExecutor{}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	if err := c.EnableSamba(context.Background(), "alice"); err != nil {
		t.Fatalf("EnableSamba falhou: %v", err)
	}
	if fe.calls[0].args[2] != "enable-samba" {
		t.Errorf("esperava enable-samba, obtido %q", fe.calls[0].args[2])
	}
}

func TestDisable_Dispatch(t *testing.T) {
	fe := &fakeExecutor{}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	if err := c.Disable(context.Background(), "alice"); err != nil {
		t.Fatalf("Disable falhou: %v", err)
	}
	if fe.calls[0].args[2] != "disable" {
		t.Errorf("esperava disable, obtido %q", fe.calls[0].args[2])
	}
}

func TestRun_EmptyBinaryPathReturnsErrBinaryMissing(t *testing.T) {
	fe := &fakeExecutor{}
	c := newWithExecutor("", fe.exec)
	if err := c.Create(context.Background(), "alice"); !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("esperava ErrBinaryMissing com path vazio, obtido: %v", err)
	}
	if len(fe.calls) != 0 {
		t.Fatalf("nenhuma chamada deveria ter sido feita: %+v", fe.calls)
	}
}

func TestRun_ExitErrorIncludesStderrMessage(t *testing.T) {
	// Simula o binário devolvendo exit 1 com mensagem no stderr.
	// Como o fakeExecutor devolve um error genérico (não
	// *exec.ExitError), o client não entra no branch de ExitError —
	// ele cai no else de "no such file/not found" se a mensagem casar,
	// ou no "executando sudo" genérico. Pra testar o branch de ExitError
	// de verdade precisaríamos de um *exec.ExitError real, que só
	// acontece com osExecutor. Cobrimos então o caminho "not found"
	// (ErrBinaryMissing) e o genérico.
	fe := &fakeExecutor{errOn: "sudo", errTo: errors.New("exec: sudo: not found")}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	err := c.Create(context.Background(), "alice")
	if !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("esperava ErrBinaryMissing quando sudo não encontrado, obtido: %v", err)
	}
}

func TestRun_GenericExecErrorWrapped(t *testing.T) {
	fe := &fakeExecutor{errOn: "sudo", errTo: errors.New("permission denied")}
	c := newWithExecutor("/opt/xvpn/bin/xvpn-user-provision", fe.exec)
	err := c.Create(context.Background(), "alice")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "executando sudo") {
		t.Errorf("erro deveria wrap 'executando sudo', obtido: %v", err)
	}
}
