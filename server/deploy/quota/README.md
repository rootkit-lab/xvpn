# Quotas de disco (Fase 15)

O painel grava `disk_quota_mb` no usuário e o `xvpn-user-provision set-quota`
chama `setquota` no VPS. Isso **só funciona** com `usrquota` ativo no
filesystem que contém `/home`.

## Ativar no VPS (produção, uma vez)

```bash
apt-get install -y quota
# Em /etc/fstab, na linha de `/` (ext4), acrescente usrquota às options, ex.:
# UUID=… / ext4 defaults,discard,usrquota 0 1
mount -o remount,usrquota /
quotacheck -cum /
quotaon -uv /
quotaon -p /   # deve mostrar user quota on
```

Validação:

```bash
setquota -u rootkit 0 102400 0 0 /   # hard 100 MiB
quota -u rootkit
```

Depois disso, o diálogo **Acesso a arquivos** do painel consegue aplicar
quotas sem erro. Sem `quotaon`, o `set-quota` falha e o painel mostra o
stderr do binário.
