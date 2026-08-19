# MongoDB no VPS (Fase 28)

`mongod` só em `127.0.0.1:27017`. Auth + user `xvpn`. Sem porta no ufw. FileBrowser Quantum (SQLite próprio) **não** entra nesta migração.

## Instalar (produção)

```bash
# read-only primeiro
ss -tulnp | grep 27017 || true
ufw status
```

Instale o pacote oficial da MongoDB Inc. (ou o do Ubuntu, se aceitável). Conf: `server/deploy/mongo/mongod.conf` — `bindIp: 127.0.0.1`, `authorization: enabled`.

```bash
mongosh --eval 'db.getSiblingDB("admin").createUser({user:"xvpn",pwd:"GERE_UMA_SENHA",roles:[{role:"readWrite",db:"xvpn"}]})'
```

URI no `/opt/xvpn/xvpn-server.env` (nunca no Git):

```
XVPN_MONGO_URI=mongodb://xvpn:SENHA@127.0.0.1:27017/xvpn?authSource=admin
```

## Migrar SQLite → Mongo

Com o server **parado** (ou em janela de manutenção):

```bash
sudo -u xvpn XVPN_MONGO_URI='...' /opt/xvpn/bin/xvpn-migrate-mongo /opt/xvpn/data/xvpn.db
```

No boot seguinte, `store.Open` vê a URI, importa se o Mongo ainda estiver vazio, e usa cache GORM em memória com mirror.

## Backup

`server/deploy/backup.sh` chama `mongodump` quando `XVPN_MONGO_URI` está set. Rotação 7 dias em `/opt/xvpn/backups/mongo/`.

## Dev local

O Mongo que você já tem: `XVPN_MONGO_URI=mongodb://127.0.0.1:27017/xvpn`. Testes/CI **não** setam a URI — continuam no SQLite.
