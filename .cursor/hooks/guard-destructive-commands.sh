#!/bin/bash
# Hook beforeShellExecution — bloqueia padrões de comando claramente destrutivos
# antes que rodem, especialmente relevante porque este projeto opera diretamente
# num VPS de produção real (206.189.224.72). Ver AGENTS.md e SECURITY.md.
#
# Filosofia: falso positivo aqui só custa uma pergunta extra ao usuário;
# falso negativo pode destruir o servidor de produção. Preferimos bloquear.

input=$(cat)
command=$(echo "$input" | jq -r '.command // empty' 2>/dev/null)

if [ -z "$command" ]; then
  echo '{"continue": true, "permission": "allow"}'
  exit 0
fi

danger_patterns=(
  'rm[[:space:]]+-[a-zA-Z]*r[a-zA-Z]*f[a-zA-Z]*[[:space:]]+/([[:space:]]|$)'
  'rm[[:space:]]+-[a-zA-Z]*f[a-zA-Z]*r[a-zA-Z]*[[:space:]]+/([[:space:]]|$)'
  'mkfs\.'
  'dd[[:space:]]+.*of=/dev/(sd|nvme|vd)'
  '>[[:space:]]*/dev/(sd|nvme|vd)'
  'ufw[[:space:]]+disable'
  'ufw[[:space:]]+--force[[:space:]]+reset'
  'iptables[[:space:]]+(-F|--flush)'
  'nft[[:space:]]+flush[[:space:]]+ruleset'
  'DROP[[:space:]]+DATABASE'
  'DROP[[:space:]]+TABLE'
  ':\(\)[[:space:]]*\{[[:space:]]*:\|:&[[:space:]]*\}[[:space:]]*;[[:space:]]*:'
  'shred[[:space:]]+.*-[a-zA-Z]*u'
  'wg-quick[[:space:]]+down[[:space:]]+wg0.*&&.*(rm|dd)'
)

for pattern in "${danger_patterns[@]}"; do
  if echo "$command" | grep -qiE "$pattern"; then
    escaped_cmd=$(echo "$command" | sed 's/"/\\"/g')
    cat <<EOF
{
  "continue": true,
  "permission": "deny",
  "agent_message": "Comando bloqueado pelo hook de segurança do XVPN por corresponder a um padrão potencialmente destrutivo ('$pattern'). Se este comando é realmente necessário e intencional, explique ao usuário exatamente o que ele faz e o impacto no VPS de produção (206.189.224.72) antes de pedir para ele rodar manualmente ou ajustar o hook.",
  "user_message": "Comando potencialmente destrutivo bloqueado automaticamente: $escaped_cmd"
}
EOF
    exit 0
  fi
done

echo '{"continue": true, "permission": "allow"}'
