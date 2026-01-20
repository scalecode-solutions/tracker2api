# mvchat2 Server Instructions

## Directory Structure

```
/srv/docker/
├── mvchat2/          # Chat server + database
│   └── docker-compose.yml
├── clingyapi/        # Pregnancy API (connects to mvchat2-db)
│   └── docker-compose.yml
└── MVCHAT2_INSTRUCTIONS.md   # This file
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    mvchat2-net                          │
│                                                         │
│  ┌──────────────┐              ┌──────────────┐        │
│  │  mvchat2-db  │              │  mvchat2-srv │        │
│  │  (postgres)  │◄────────────►│ (chat server)│        │
│  │  port 5433   │              │  port 6061   │        │
│  └──────────────┘              └──────────────┘        │
│         ▲                                              │
│         │                                              │
│  ┌──────────────┐                                      │
│  │  clingyapi   │                                      │
│  │  port 6062   │                                      │
│  └──────────────┘                                      │
└─────────────────────────────────────────────────────────┘
```

## Containers

| Container   | Port | Purpose                    |
|-------------|------|----------------------------|
| mvchat2-db  | 5433 | PostgreSQL database        |
| mvchat2-srv | 6061 | WebSocket chat server      |
| clingyapi   | 6062 | Pregnancy tracking REST API|

## Volumes

| Volume          | Purpose                      |
|-----------------|------------------------------|
| mvchat2-data    | PostgreSQL data              |
| mvchat2-uploads | File uploads (shared)        |

## Network

| Network     | Purpose                      |
|-------------|------------------------------|
| mvchat2-net | Internal communication       |

## Commands

### Start Everything (in order)

```bash
# 1. Start mvchat2 (db + server)
cd /srv/docker/mvchat2
docker compose up -d

# 2. Start ClingyAPI (after mvchat2 is healthy)
cd /srv/docker/clingyapi
docker compose up -d
```

### Stop Everything

```bash
cd /srv/docker/clingyapi && docker compose down
cd /srv/docker/mvchat2 && docker compose down
```

### Rebuild After Code Changes

```bash
# Rebuild mvchat2
cd /srv/docker/mvchat2
docker compose up -d --build

# Rebuild ClingyAPI
cd /srv/docker/clingyapi
docker compose up -d --build
```

### View Logs

```bash
docker logs mvchat2-srv -f    # Chat server
docker logs clingyapi -f       # Pregnancy API
docker logs mvchat2-db -f      # Database
```

### Check Status

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'mvchat2|clingy'
```

### Run ClingyAPI Migrations (fresh install only)

```bash
for f in /srv/docker/clingyapi/migrations/*.sql; do
  docker exec -i mvchat2-db psql -U mvchat2 -d mvchat2 < "$f"
done
```

### Database Access

```bash
docker exec -it mvchat2-db psql -U mvchat2 -d mvchat2
```

## Important Notes

1. **Always use docker compose** - Never use `docker run` directly. This ensures proper volume and network naming.

2. **Start order matters** - mvchat2 must be running before ClingyAPI starts (ClingyAPI needs mvchat2-db and mvchat2-net).

3. **Shared resources**:
   - Volume `mvchat2-uploads` is shared between mvchat2-srv and clingyapi
   - Network `mvchat2-net` is created by mvchat2, used by clingyapi as external

4. **Token key**: Both services use the same JWT token key:
   `pyn/SxK8gwZEzVyMKNY1tp0jp2NTaSXiJq5FGOIJp5U=`

## Caddy Routes

| Domain             | Port | Service    |
|--------------------|------|------------|
| api2.mvchat.app    | 6061 | mvchat2-srv|
| clingy.mvchat.app  | 6062 | clingyapi  |

## Troubleshooting

### ClingyAPI can't connect to database
```bash
# Check if on same network
docker network inspect mvchat2-net

# Verify connectivity
docker exec clingyapi ping mvchat2-db
```

### Container using wrong volume
Always stop with `docker compose down`, never just `docker stop`.
If needed, remove and recreate:
```bash
docker compose down -v  # Removes volumes too!
docker compose up -d
```

---

## Remote Access (from local machine)

**SSH**: `ssh -p 62722 root@scalecode.dev`

**View Logs**:
```bash
ssh -p 62722 root@scalecode.dev "docker logs mvchat2-srv -f"
ssh -p 62722 root@scalecode.dev "docker logs clingyapi -f"
```

**Restart Services**:
```bash
ssh -p 62722 root@scalecode.dev "cd /srv/docker/mvchat2 && docker compose restart"
ssh -p 62722 root@scalecode.dev "cd /srv/docker/clingyapi && docker compose restart"
```

**Server Location**: `/srv/docker/mvchat2/` on scalecode.dev
