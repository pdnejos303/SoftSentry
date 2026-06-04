# 08 — Deployment

## Target environment

**v1 deployment model:** single VM (4 vCPU / 8 GB RAM / 100 GB SSD) running Docker + Compose.
สำหรับ < 1,000 endpoints. Scale-out plan ไว้ v2

OS recommended: Ubuntu 22.04 LTS หรือ Debian 12

---

## Pre-deployment checklist

- [ ] Domain name + DNS A record → server IP
- [ ] Firewall: เปิด port 80 + 443 ขาเข้า, อื่นๆ block
- [ ] TLS cert: Let's Encrypt via Certbot (หรือ corporate CA)
- [ ] Database backup destination (S3 bucket หรือ NAS)
- [ ] Monitoring access — Grafana port forward หรือ separate subdomain
- [ ] Secrets generated:
  - `JWT_SECRET` (32 bytes hex)
  - `LICENSE_KEY_ENCRYPTION_KEY` (32 bytes hex)
  - `POSTGRES_PASSWORD` (strong)
  - `REDIS_PASSWORD` (strong)
- [ ] Initial admin email + password
- [ ] NVD API key (optional but recommended)

---

## Step-by-step

### 1. Server prep
```bash
# As root or sudo user on Ubuntu 22.04
apt update && apt upgrade -y
apt install -y docker.io docker-compose-plugin git ufw certbot

# Firewall
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable

# Create deploy user
useradd -m -s /bin/bash softsentry
usermod -aG docker softsentry
su - softsentry
```

### 2. Clone + configure
```bash
cd /home/softsentry
git clone <repo> app
cd app

cp .env.example .env
# Edit .env — set all secrets to strong values, ENV=production, ENABLE_DOCS=false
chmod 600 .env
```

### 3. TLS cert
```bash
# Let's Encrypt (assuming domain points to this server)
sudo certbot certonly --standalone -d softsentry.yourcompany.com

# Symlink into nginx volume
sudo mkdir -p /home/softsentry/app/infra/nginx/certs
sudo ln -s /etc/letsencrypt/live/softsentry.yourcompany.com/fullchain.pem /home/softsentry/app/infra/nginx/certs/cert.pem
sudo ln -s /etc/letsencrypt/live/softsentry.yourcompany.com/privkey.pem /home/softsentry/app/infra/nginx/certs/key.pem

# Auto-renew cron
echo "0 3 * * * certbot renew --quiet --post-hook 'docker compose -f /home/softsentry/app/docker-compose.prod.yml restart nginx'" | sudo crontab -
```

### 4. Build + start
```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml exec backend alembic upgrade head
docker compose -f docker-compose.prod.yml exec backend python -m app.seed
```

### 5. Verify
```bash
curl -k https://softsentry.yourcompany.com/api/v1/health
# {"status": "ok", "db": "ok", "redis": "ok", "version": "1.0.0"}

# Open browser → https://softsentry.yourcompany.com → login with initial admin
```

---

## docker-compose.prod.yml (differences from dev)

- `restart: unless-stopped` on ทุก service
- No `develop.watch` (no hot reload)
- Backend run via `gunicorn -w 4 -k uvicorn.workers.UvicornWorker app.main:app`
- Dashboard build with `pnpm build` → run with `pnpm start`
- No port exposure for postgres/redis to host — internal only
- Volume bind mounts → named volumes
- Resource limits (memory/cpu) on each service
- Logs → JSON driver with rotation

---

## Database migrations in production

```bash
# Pull new code
git pull

# Build new images
docker compose -f docker-compose.prod.yml build

# Run migrations BEFORE starting new containers (zero-downtime check)
docker compose -f docker-compose.prod.yml run --rm backend alembic upgrade head

# Start new containers
docker compose -f docker-compose.prod.yml up -d
```

### Migration safety
- **อ่าน migration ก่อน apply** ใน production เสมอ
- ถ้ามี `DROP COLUMN` หรือ `ALTER TYPE` → schedule maintenance window
- Backup ก่อน apply เสมอ (เห็นด้านล่าง)

---

## Backup & Restore

### Daily automated backup
```bash
# /usr/local/bin/softsentry-backup.sh
#!/bin/bash
set -e
BACKUP_DIR=/var/backups/softsentry
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

docker compose -f /home/softsentry/app/docker-compose.prod.yml exec -T postgres \
  pg_dump -U softsentry softsentry | gzip > $BACKUP_DIR/db_$DATE.sql.gz

# Encrypt
gpg --encrypt -r backup@yourcompany.com $BACKUP_DIR/db_$DATE.sql.gz

# Upload to S3
aws s3 cp $BACKUP_DIR/db_$DATE.sql.gz.gpg s3://your-backup-bucket/softsentry/

# Cleanup local > 7 days
find $BACKUP_DIR -mtime +7 -delete
```

Cron: `0 2 * * * /usr/local/bin/softsentry-backup.sh`

### Restore
```bash
# Stop services
docker compose -f docker-compose.prod.yml stop backend worker dashboard

# Restore
gunzip < backup.sql.gz | docker compose exec -T postgres psql -U softsentry softsentry

# Restart
docker compose -f docker-compose.prod.yml start backend worker dashboard
```

---

## Monitoring

### Health endpoint
`GET /api/v1/health` — return 200 ถ้า DB + Redis reachable

### Uptime monitoring
- Use UptimeRobot / Statuscake / self-hosted Uptime Kuma
- Ping `/api/v1/health` ทุก 1 นาที, alert ถ้า fail 3 ครั้งติด

### Logs
```bash
docker compose -f docker-compose.prod.yml logs -f --tail 100 backend
```

For aggregation (optional): Loki + Promtail in same Docker network, Grafana datasource

### Metrics
- Backend export `/metrics` (Prometheus format) — `prometheus-fastapi-instrumentator`
- Agent push metrics ผ่าน API → backend forward เป็น metrics (TBD ใน Phase 5)
- Grafana dashboards provisioned ใน `infra/grafana/dashboards/`

### Alerts (Phase 5+)
Grafana alerting:
- Backend down > 1 min
- DB connection failures > 5 / min
- Worker queue depth > 100
- p95 latency > 2s
- Agent fleet: > 10% offline

---

## Updating

```bash
cd /home/softsentry/app
git fetch
git log HEAD..origin/main      # review changes
git pull

# Read migration changes if any
ls backend/alembic/versions/

# Backup
/usr/local/bin/softsentry-backup.sh

# Build + migrate + restart
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml run --rm backend alembic upgrade head
docker compose -f docker-compose.prod.yml up -d
```

Rollback (if migration is reversible):
```bash
docker compose -f docker-compose.prod.yml run --rm backend alembic downgrade -1
git checkout <previous-tag>
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

---

## Agent distribution

### Build matrix
```bash
# Build for all platforms
make -C agent release
# → agent/dist/softsentry-agent-windows-amd64.exe
# → agent/dist/softsentry-agent-darwin-amd64
# → agent/dist/softsentry-agent-darwin-arm64
```

### Code signing
- **Windows**: sign with Authenticode cert (EV preferred to avoid SmartScreen)
- **macOS**: sign with Apple Developer ID + notarize via `xcrun notarytool`

### Distribution to endpoints
- Host signed binaries on backend (`GET /api/v1/agents/binary/...`)
- IT distributes installer via SCCM / Jamf / manual

### Installer
- **Windows**: MSI built with WiX. Install service, set ACL on `%ProgramData%\SoftSentry`
- **macOS**: PKG built with `pkgbuild`. Install LaunchDaemon plist to `/Library/LaunchDaemons/`

---

## Disaster recovery

| Scenario | Action |
|----------|--------|
| Server lost | Restore from latest DB backup → re-enroll all agents (worst case) |
| DB corrupted | Restore from backup, max data loss = 24 hr scans |
| Lost JWT_SECRET | All users logged out, no data loss, regenerate + restart |
| Lost LICENSE_KEY_ENCRYPTION_KEY | License keys unrecoverable. Have offline backup of this key |
| Agent compromised | Revoke token in DB, reinstall agent |

---

## Performance tuning (when needed)

| Symptom | Action |
|---------|--------|
| Slow dashboard | Postgres `EXPLAIN ANALYZE` → add index. Check `pg_stat_statements` |
| High CPU on backend | Increase gunicorn workers (`-w 4` → `-w 8`) |
| Worker queue backlog | Add another worker container |
| Slow scan submit | Postgres connection pool size (`pool_size=20` in DATABASE_URL options) |

ไม่ optimize ก่อน measure — ใช้ Grafana dashboards
