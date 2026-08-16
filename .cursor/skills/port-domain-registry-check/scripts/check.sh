#!/bin/bash
# Compara o registro de portas/domínios documentado em PLAN.md §5 com o que
# está de fato escutando no servidor XVPN.
# Uso: check.sh [usuario@host]
set -euo pipefail

HOST="${1:-root@206.189.224.72}"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
PLAN_FILE="$REPO_ROOT/PLAN.md"

echo "===== Registro documentado (PLAN.md §5) ====="
if [ -f "$PLAN_FILE" ]; then
  awk '/^## 5\. Alocação de rede/{flag=1} flag; /^## 6\./{flag=0}' "$PLAN_FILE"
else
  echo "PLAN.md não encontrado em $REPO_ROOT — rode este script a partir do repositório do projeto."
fi

echo
echo "===== Estado real no servidor ($HOST) ====="
ssh -o ConnectTimeout=10 "$HOST" "ss -tulnp 2>/dev/null || ss -tuln"

echo
echo "===== Lembretes (PLAN.md §5 / runbook Cloudflare) ====="
echo "- Público: 22, 80, 443, 51820/udp. Sem 53, 445, 27017, 8080, 8081 na eth0."
echo "- Intranet: *.corp só resolve em 10.66.66.1:53 (dnsmasq no wg0). Sem A público para corp."
echo "- Mongo: só 127.0.0.1:27017. DNS interno: só 10.66.66.1:53."
echo
echo "Compare: porta pública no ss ⇔ linha na tabela. Bind documentado como 127.0.0.1 ou 10.66.66.1 não pode aparecer em 0.0.0.0 nem no IP público."
echo "Runbook: docs/runbooks/cloudflare-dns.md"
