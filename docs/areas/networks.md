# Redes overlay

Um `wg0` no hub (`51820/udp`). Várias faixas. Sem segundo listen público.

| Rede | Kind | CIDR | Quem |
|---|---|---|---|
| `infra` | infra | `10.66.66.0/24` | control `.1`, malha/data, VIP `.254` |
| `users` | users | `10.66.80.0/24` | devices no enroll (`exit=true`) |
| custom | custom | fatias de `10.66.80.0/20` | times/labs no xadmin |

UI: `xadmin.corp` → **Redes** (`/admin/networks`). Produto `core`. Compute cadastra **hosts**; o peer mesh entra em `infra`.

## Semântica

- Mesma rede: L3 entre membros.
- Entre redes: FORWARD **nega**, salvo `NetworkRule` allow. Membership só dá rota (`AllowedIPs`); não abre portas.
- Seed: `users` → `infra` TCP 443/53, UDP 53, TCP 445 (`samba`). **Não** 27017.
- Device enroll → `users`. Mesh enroll → `infra` + `AllowedIPs` = CIDR infra (sem `0.0.0.0/0`).
- Seed no boot **move** device de usuário que ainda estiver em `10.66.66.x` para `users`. Cliente precisa reconectar/re-enroll.

## Apply

`xvpn-user-provision overlay-apply` grava `/etc/xvpn/overlay.nft` (`nft -f`). NAT de exit só para redes com `exit=true` (`oif != wg0`).
