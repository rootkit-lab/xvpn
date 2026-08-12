package wireguard

import "testing"

func TestAllocateIP_FirstFree(t *testing.T) {
	ip, err := AllocateIP("10.66.66.0/24", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// .1 é reservado ao servidor, então o primeiro peer deve ficar em .2.
	if ip != "10.66.66.2/32" {
		t.Fatalf("esperado 10.66.66.2/32, obtido %s", ip)
	}
}

func TestAllocateIP_SkipsUsedAndReserved(t *testing.T) {
	used := []string{"10.66.66.2/32", "10.66.66.3/32", "10.66.66.1/32"}
	ip, err := AllocateIP("10.66.66.0/24", used)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if ip != "10.66.66.4/32" {
		t.Fatalf("esperado 10.66.66.4/32, obtido %s", ip)
	}
}

func TestAllocateIP_AcceptsPlainIPInUsedList(t *testing.T) {
	used := []string{"10.66.66.2"}
	ip, err := AllocateIP("10.66.66.0/24", used)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if ip != "10.66.66.3/32" {
		t.Fatalf("esperado 10.66.66.3/32, obtido %s", ip)
	}
}

func TestAllocateIP_ExhaustedSubnet(t *testing.T) {
	// /30 só tem 2 hosts utilizáveis (.1 e .2); .1 é reservado ao servidor
	// e .2 é ocupado, então não deve sobrar nenhum IP livre.
	used := []string{"10.66.66.2/32"}
	_, err := AllocateIP("10.66.66.0/30", used)
	if err == nil {
		t.Fatalf("esperava erro de sub-rede esgotada, obteve nil")
	}
}

func TestAllocateIP_InvalidSubnet(t *testing.T) {
	_, err := AllocateIP("not-a-cidr", nil)
	if err == nil {
		t.Fatalf("esperava erro de sub-rede inválida, obteve nil")
	}
}
