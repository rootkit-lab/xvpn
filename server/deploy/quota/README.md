# Quotas de disco (Fase 15)

O painel grava `disk_quota_mb` no usuário e o `xvpn-user-provision set-quota`
chama `setquota` no VPS. Isso **só funciona** com `usrquota` ativo no
filesystem que contém `/home`.

## Ativar no VPS (produção, uma vez)

```bash
apt-get install -y quota
# Em /etc/fstab, na linha de `/` (ext4), acrescente usrquota às options, ex.:
# LABEL=cloudimg-rootfs  /  ext4  discard,commit=30,errors=remount-ro,usrquota  0 1
mount -o remount,usrquota /
quotacheck -cum /
quotaon -uv /
systemctl enable quota.service   # liga no boot
quotaon -p /   # deve mostrar "user quota on"
```

Avisos esperados e inofensivos:
- `Cannot stat() mounted device tmpfs` — o utilitário `quota` varre `/proc/mounts` e reclama de tmpfs; ignore.
- `external quota files on ext4 are deprecated` — o kernel sugere `tune2fs -O quota` (quota journalizada), que exige desmontar `/`. Em root live ficamos com `aquota.user` + `usrquota` no fstab, que funciona.

## Validação

```bash
setquota -u nobody 0 102400 0 0 /   # hard 100 MiB
quota -vs nobody                    # limit deve mostrar 100M
setquota -u nobody 0 0 0 0 /        # limpa o teste
```

Depois disso, o diálogo **Acesso a arquivos** do painel consegue aplicar
quotas sem erro. Sem `quotaon`, o `set-quota` falha e o painel mostra o
stderr do binário.
