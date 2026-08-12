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
echo "Compare manualmente: toda porta pública em uso no servidor deve ter uma linha correspondente na tabela acima. Toda porta documentada como 'interna' (127.0.0.1) ou 'somente VPN' (10.66.66.1) não deve aparecer vinculada a 0.0.0.0 ou ao IP público na saída do ss."
