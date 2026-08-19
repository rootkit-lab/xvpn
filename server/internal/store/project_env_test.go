package store

import "testing"

func TestValidProjectEnvName(t *testing.T) {
	oks := []string{"APP_URL", "XCS_LLM_KEY", "A1", "FOO_BAR_2"}
	for _, n := range oks {
		if !ValidProjectEnvName(n) {
			t.Fatalf("deveria aceitar %q", n)
		}
	}
	bads := []string{"", "a", "PATH ", "1FOO", "foo", "A", "app-url"}
	for _, n := range bads {
		if ValidProjectEnvName(n) {
			t.Fatalf("deveria rejeitar %q", n)
		}
	}
}

func TestBlockedAndLLMProjectEnv(t *testing.T) {
	if !BlockedProjectEnvName("PATH") || !BlockedProjectEnvName("HOME") {
		t.Fatal("PATH/HOME bloqueados")
	}
	if !BlockedProjectEnvName("LD_LIBRARY_PATH") || !BlockedProjectEnvName("SSH_AUTH_SOCK") || !BlockedProjectEnvName("DOCKER_HOST") {
		t.Fatal("prefixos bloqueados")
	}
	if !BlockedProjectEnvName("NODE_OPTIONS") || !BlockedProjectEnvName("BASH_ENV") || !BlockedProjectEnvName("PROMPT_COMMAND") {
		t.Fatal("runtime-control bloqueado")
	}
	if !BlockedProjectEnvName("PS1") || !BlockedProjectEnvName("PS4") {
		t.Fatal("PS1/PS4 bloqueados")
	}
	for _, n := range []string{"IFS", "CDPATH", "GCONV_PATH", "DOTNET_STARTUP_HOOKS", "GLIBC_TUNABLES"} {
		if !BlockedProjectEnvName(n) {
			t.Fatalf("deveria bloquear %s", n)
		}
	}
	if BlockedProjectEnvName("APP_URL") {
		t.Fatal("APP_URL livre")
	}
	if !IsLLMProjectEnv("XCS_LLM_KEY") || IsLLMProjectEnv("APP_URL") {
		t.Fatal("XCS_LLM_*")
	}
	if ValidProjectEnvValue("a\nb") || !ValidProjectEnvValue("ok") {
		t.Fatal("valor")
	}
}
