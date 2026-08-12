package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3nh4-forte-de-teste")
	if err != nil {
		t.Fatalf("erro inesperado ao gerar hash: %v", err)
	}

	ok, err := VerifyPassword(hash, "s3nh4-forte-de-teste")
	if err != nil {
		t.Fatalf("erro inesperado ao verificar hash: %v", err)
	}
	if !ok {
		t.Fatalf("senha correta não foi reconhecida como válida")
	}

	ok, err = VerifyPassword(hash, "senha-errada")
	if err != nil {
		t.Fatalf("erro inesperado ao verificar hash: %v", err)
	}
	if ok {
		t.Fatalf("senha errada foi aceita como válida")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	_, err := VerifyPassword("nao-e-um-hash-argon2id", "qualquer-coisa")
	if err == nil {
		t.Fatalf("esperava erro para hash malformado, obteve nil")
	}
}

func TestHashPassword_ProducesDifferentSaltsEachTime(t *testing.T) {
	h1, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	h2, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("dois hashes da mesma senha não deveriam ser idênticos (salt deve variar)")
	}
}
