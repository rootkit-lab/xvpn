# Restore de backup off-site (Fase 44)

O `server/deploy/backup.sh` copia no **mesmo disco**. Off-site é restic (+ rclone para Drive/WebDAV), configurado em `xadmin.corp` → **Backups**. Credenciais nunca entram no Git.

## O que o job inclui

Opt-in na UI: Mongo (`mongodump`), blobs do marketplace, bare do forge (`/opt/xvpn/data/git`), mídia social. XDRIVER é cópia extra no share (`/srv/xvpn/shared/xvpn-backups/…`) — **não** substitui off-site e **nunca** leva o dump do Mongo (o `[shared]` é guest).

## Restore restic (SFTP / B2 / S3 / rclone)

No VPS, com a mesma senha do repositório (a que foi gravada no destino — só no Mongo):

```bash
export RESTIC_REPOSITORY='sftp:user@host:/path'   # ou b2:bucket:path / s3:bucket / rclone:remote:path
export RESTIC_PASSWORD='…'                        # nunca commitar
# B2/S3: B2_ACCOUNT_* ou AWS_* como no job
restic snapshots
restic restore latest --target /tmp/xvpn-restore
```

Mongo: o dump entra como pasta `mongodump` no snapshot. Restaurar com `mongorestore --uri="$XVPN_MONGO_URI" /tmp/xvpn-restore/...`.

Forge: copiar os `*.git` de volta para `/opt/xvpn/data/git/` e `chown -R xvpn:xvpn`.

Marketplace / social: rsync das pastas content-addressed para `XVPN_MARKETPLACE_DIR` / `XVPN_SOCIAL_MEDIA_DIR`.

## Restore XDRIVER

Os arquivos já estão no share. Não use isso como único backup.

## Dry-run

Na UI, **Dry-run** chama `restic backup --dry-run` (ou só lista o alvo no XDRIVER). Não grava snapshot.
