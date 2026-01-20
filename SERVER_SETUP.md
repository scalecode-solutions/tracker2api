# Tracker2API Server Setup Reference

**Server:** `root@scalecode.dev`

---

## Docker Containers

| Container | Image | Status | Ports |
|-----------|-------|--------|-------|
| `mvchat-srv` | mvchat:latest | Up (healthy) | 6060:6060, 16060/tcp |
| `mvchat-db` | postgres:15 | Up (healthy) | 5432 (internal) |
| `tracker2api` | tracker2api-tracker2api | Up | 6063:6063 |
| `mvchat2-test-srv` | mvchat2-test-mvchat2 | Up (healthy) | 6061:6060 |
| `mvchat2-test-db` | postgres:18-alpine | Up (healthy) | 5432 (internal) |

**Docker Network:** `mvchat_mvchat-net` (172.19.0.0/16, bridge)
- `mvchat-db`: 172.19.0.2
- `mvchat-srv`: 172.19.0.3

---

## Directory Structure

```
/srv/docker/
├── mvchat/                    # Production mvchat
│   ├── docker-compose.yml
│   ├── Caddyfile              # OLD - not used by systemd
│   ├── server/
│   │   ├── mvchat.conf        # Main config (use this, not tinode.conf)
│   │   └── ...
│   └── docker/
├── Tracker2API/               # Tracker2API (cloned from GitHub)
│   ├── docker-compose.yml
│   ├── Dockerfile
│   ├── cmd/server/main.go
│   ├── internal/
│   ├── data/                  # Static JSON data files
│   │   ├── BabySizes.json
│   │   └── WeeklyFacts.json
│   └── migrations/
├── mvchat2-test/              # Test instance
└── filesapi/

/etc/caddy/
└── Caddyfile                  # ACTUAL Caddy config used by systemd
```

---

## PostgreSQL Database

### Connection Details

| Property | Value |
|----------|-------|
| Host (from docker network) | `db` or `mvchat-db` |
| Port | 5432 |
| User | `mvchat` |
| Password | `tinode_secure_2024` |
| Database | `mvchat` |
| SSL Mode | disable |

**Connection String (for Tracker2API from within docker network):**
```
postgresql://mvchat:tinode_secure_2024@mvchat-db:5432/mvchat?sslmode=disable
```

### Database Roles

| Role | Attributes |
|------|------------|
| `mvchat` | Superuser, Create role, Create DB, Replication, Bypass RLS |
| `tinode` | Superuser, Create role, Create DB, Replication, Bypass RLS |

**Note:** Tables are owned by `tinode` user, but `mvchat` user has full access.

### mvchat Tables (14, owned by tinode)

| Table | Purpose |
|-------|---------|
| `users` | User accounts |
| `auth` | Authentication records |
| `credentials` | Login credentials |
| `topics` | Chat topics/conversations |
| `members` | Topic memberships |
| `messages` | Chat messages |
| `dellog` | Deletion log |
| `devices` | Push notification devices |
| `fileuploads` | Uploaded files |
| `file_metadata` | File metadata |
| `filemsglinks` | File-message associations |
| `kvmeta` | Key-value metadata (includes DB version) |
| `topictags` | Topic tags |
| `usertags` | User tags |

### Tracker2API Tables (6, created by migration)

| Table | Purpose |
|-------|---------|
| `clingy_pregnancies` | Pregnancy records (one per owner) |
| `clingy_entries` | All tracker entries (weight, symptoms, etc.) |
| `clingy_settings` | User settings per pregnancy |
| `clingy_files` | Uploaded file metadata |
| `clingy_pairing_requests` | Partner access requests |
| `clingy_sync_state` | Per-device sync tracking |

### Test Users

| ID | Name | Email | Password |
|----|------|-------|----------|
| 1482926326842134528 | Travis | tsrlegends@gmail.com | Echelon1! |
| 1482926200593584128 | Shelby | shelby.cottrell@yahoo.com | DrPepper91 |

---

## Authentication Configuration

From `/srv/docker/mvchat/server/mvchat.conf`:

```json
"auth_config": {
  "token": {
    "expire_in": 1209600,
    "serial_num": 1,
    "key": "wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc="
  }
}
```

| Property | Value |
|----------|-------|
| **AUTH_TOKEN_KEY** | `wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=` |
| **AUTH_SERIAL_NUM** | `1` |
| Token Expiry | 1209600 seconds (2 weeks) |

### Token Binary Format (50 bytes)

```
[8:UserID][4:Expires][2:AuthLevel][2:SerialNum][2:Features][32:HMAC-SHA256]
```

### Generate Test Token (Go)

```go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

type tokenLayout struct {
	UserId       uint64
	Expires      uint32
	AuthLevel    uint16
	SerialNumber uint16
	Features     uint16
}

func main() {
	key, _ := base64.StdEncoding.DecodeString("wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=")

	// Travis user ID
	userID := uint64(1482926326842134528)

	tl := tokenLayout{
		UserId:       userID,
		Expires:      uint32(time.Now().Add(24 * time.Hour).Unix()),
		AuthLevel:    20,
		SerialNumber: 1,
		Features:     0,
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, &tl)

	hasher := hmac.New(sha256.New, key)
	hasher.Write(buf.Bytes())
	binary.Write(buf, binary.LittleEndian, hasher.Sum(nil))

	fmt.Println(base64.StdEncoding.EncodeToString(buf.Bytes()))
}
```

---

## File Storage

### mvchat Uploads

| Property | Value |
|----------|-------|
| Docker Volume | `mvchat_mvchat-uploads` |
| Host Path | `/var/lib/docker/volumes/mvchat_mvchat-uploads/_data/` |
| Container Path | `/app/uploads` |

**Note:** There is NO `/srv/docker/mvchat/uploads` directory. Files are stored in Docker volume only.

---

## Caddy Configuration

**Version:** 2.10.2
**Config File:** `/etc/caddy/Caddyfile` (NOT `/srv/docker/mvchat/Caddyfile`)
**Managed by:** systemd (`systemctl reload caddy`)

### mvchat Domains

| Domain | Target | Purpose |
|--------|--------|---------|
| `api.mvchat.app` | localhost:6060 | mvchat API (production) |
| `api.clingy.me` | localhost:6063 | **Tracker2API** |
| `test.mvchat.app` | localhost:6061 | mvchat test environment |
| `api2.mvchat.app` | localhost:6061 | mvchat2 test API |
| `mvchat.app` | - | Landing page placeholder |
| `degenerates.dev` | localhost:6060 | mvchat legacy |

### Tracker2API Caddy Config (already in /etc/caddy/Caddyfile)

```caddy
api.clingy.me {
    log {
        output file /var/log/caddy/api.clingy.me.log
        format json
    }
    import security_headers
    import api_rate_limit
    reverse_proxy localhost:6063 {
        transport http {
            dial_timeout 10s
            response_header_timeout 30s
        }
    }
    request_body {
        max_size 200MB
    }
}
```

### Global Snippets Available

- `security_headers` - HSTS, X-Content-Type-Options, X-Frame-Options, etc.
- `api_rate_limit` - 100 requests/minute per IP
- `auth_rate_limit` - 10 requests/minute per IP (stricter)
- `wp_honeypot` - WordPress scanner trap
- `cors_headers` - CORS for chat.mvchat.app

---

## Tracker2API Deployment

### GitHub Repository

**URL:** `https://github.com/scalecode-solutions/Tracker2API.git`

### Deploy Key (on server)

```
Location: ~/.ssh/tracker2api_deploy
Public Key: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIUfYtlQ8u0/Dzj6oVdIc5ORyKAFETi50oXIEEBySma2 tracker2api-deploy-key
```

SSH Config (`~/.ssh/config`):
```
Host github.com-tracker2api
    HostName github.com
    User git
    IdentityFile ~/.ssh/tracker2api_deploy
    IdentitiesOnly yes
```

### Initial Deployment Steps

```bash
# 1. Clone repo (uses deploy key)
ssh root@scalecode.dev "cd /srv/docker && git clone git@github.com-tracker2api:scalecode-solutions/Tracker2API.git"

# 2. Run database migration
ssh root@scalecode.dev "docker exec -i mvchat-db psql -U mvchat -d mvchat < /srv/docker/Tracker2API/migrations/001_initial.sql"

# 3. Build and start container
ssh root@scalecode.dev "cd /srv/docker/Tracker2API && docker compose up -d --build"

# 4. Verify
curl https://api.clingy.me/health
```

### Update Deployment

```bash
ssh root@scalecode.dev "cd /srv/docker/Tracker2API && git pull && docker compose up -d --build"
```

### Environment Variables (in docker-compose.yml)

```env
PORT=6063
DATABASE_URL=postgresql://mvchat:tinode_secure_2024@mvchat-db:5432/mvchat?sslmode=disable
AUTH_TOKEN_KEY=wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=
AUTH_SERIAL_NUM=1
UPLOAD_PATH=/app/uploads/tracker2
```

---

## API Testing

### Health Check

```bash
curl https://api.clingy.me/health
# Response: OK
```

### Authenticated Requests

```bash
# Generate token first (see Go code above), then:
TOKEN="<base64-token>"

# Get pregnancy
curl -H "Authorization: Bearer $TOKEN" https://api.clingy.me/api/pregnancy

# Create pregnancy
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  https://api.clingy.me/api/pregnancy \
  -d '{"dueDate":"2025-06-15","babyName":"Baby","momName":"Mom","parentRole":"mother"}'

# Create entry
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  https://api.clingy.me/api/entries \
  -d '{"clientId":"1736711000000","entryType":"weight","data":{"date":"2025-01-12","weight":145.5}}'

# Get entries
curl -H "Authorization: Bearer $TOKEN" https://api.clingy.me/api/entries

# Full sync
curl -H "Authorization: Bearer $TOKEN" https://api.clingy.me/api/sync

# Pairing status
curl -H "Authorization: Bearer $TOKEN" https://api.clingy.me/api/pairing/status
```

### Verified Endpoints (2026-01-20)

| Endpoint | Method | Status |
|----------|--------|--------|
| `/health` | GET | ✓ Working |
| `/api/data/baby-sizes` | GET | New - Static JSON data |
| `/api/data/weekly-facts` | GET | New - Static JSON data |
| `/api/pregnancy` | GET | ✓ Working |
| `/api/pregnancy` | POST | ✓ Working |
| `/api/entries` | GET | ✓ Working |
| `/api/entries` | POST | ✓ Working |
| `/api/sync` | GET | ✓ Working |
| `/api/pairing/status` | GET | ✓ Working |
| `/api/settings` | GET | ✓ Working |

---

## Useful Commands

### Database Access
```bash
# Connect to database interactively
ssh root@scalecode.dev "docker exec -it mvchat-db psql -U mvchat -d mvchat"

# Run SQL command
ssh root@scalecode.dev "docker exec mvchat-db psql -U mvchat -d mvchat -c 'SELECT * FROM users;'"

# Check Tracker2API tables
ssh root@scalecode.dev "docker exec mvchat-db psql -U mvchat -d mvchat -c 'SELECT * FROM clingy_pregnancies;'"
ssh root@scalecode.dev "docker exec mvchat-db psql -U mvchat -d mvchat -c 'SELECT * FROM clingy_entries;'"
```

### Container Management
```bash
# View logs
ssh root@scalecode.dev "docker logs tracker2api --tail 100"
ssh root@scalecode.dev "docker logs mvchat-srv --tail 100"

# Restart container
ssh root@scalecode.dev "docker restart tracker2api"

# Check container status
ssh root@scalecode.dev "docker ps"

# Rebuild Tracker2API
ssh root@scalecode.dev "cd /srv/docker/Tracker2API && docker compose up -d --build"
```

### Caddy
```bash
# Reload config (after editing /etc/caddy/Caddyfile)
ssh root@scalecode.dev "systemctl reload caddy"

# Check status
ssh root@scalecode.dev "systemctl status caddy"

# View logs
ssh root@scalecode.dev "tail -f /var/log/caddy/api.clingy.me.log"
```

---

## Troubleshooting

### Token Validation Failures

**Issue:** All requests return `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`

**Cause:** AUTH_TOKEN_KEY must be base64-decoded before use.

**Fix:** Ensure `main.go` decodes the key:
```go
authKeyBytes, err := base64.StdEncoding.DecodeString(authTokenKey)
authenticator := auth.New(authKeyBytes, authSerialNum)
```

### Database Connection Issues

```bash
# Test from container
ssh root@scalecode.dev "docker exec tracker2api ping mvchat-db"

# Check network
ssh root@scalecode.dev "docker network inspect mvchat_mvchat-net"
```

### Container Won't Start

```bash
# Check logs
ssh root@scalecode.dev "docker logs tracker2api"

# Check if port is in use
ssh root@scalecode.dev "netstat -tlnp | grep 6063"
```

---

## Port Summary (mvchat ecosystem)

| Port | Service |
|------|---------|
| 6060 | mvchat production (api.mvchat.app) |
| 6061 | mvchat2 test (test.mvchat.app, api2.mvchat.app) |
| 6063 | **Tracker2API** (api.clingy.me) |

---

## Other Services on Server

| Domain | Port | Service |
|--------|------|---------|
| bettercerts.com | 5000, 5001 | BetterCerts API/Auth |
| api.bettercerts.com | 5000 | BetterCerts API |
| filesapi.dev | 8080 | FilesAPI |
| pisreports.com | 8080 | PIS Reports |
| ciscale.simplecerts.app | 8081 | CI Scale Dashboard |
| simplecerts.app | 3001 | SimpleCerts |

---

*Last updated: 2026-01-20*
*Tracker2API renamed from ClingyAPI with static data endpoints added*
