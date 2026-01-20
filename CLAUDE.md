# Tracker2API

Go REST API backend for the Clingy pregnancy tracker app. Provides data sync across devices, partner/supporter access via invite codes, and pregnancy outcome tracking.

## Tech Stack
- **Language:** Go 1.25
- **Database:** PostgreSQL 15 (shared with mvchat)
- **Authentication:** mvchat2 JWT tokens (HMAC-SHA256)
- **Router:** Gorilla Mux
- **Deployment:** Docker container on port 6062

## Commands

```bash
# Build
go build -o tracker2api ./cmd/server

# Run locally (requires env vars)
./tracker2api

# Build Docker image
docker build -t tracker2api .

# Run all migrations
for f in migrations/*.sql; do
  docker exec -i mvchat-db psql -U mvchat -d mvchat < "$f"
done

# Deploy to server
ssh root@scalecode.dev
cd /srv/docker/Tracker2API
git pull
docker build -t tracker2api .
docker stop tracker2api && docker rm tracker2api
docker run -d --name tracker2api --network mvchat_mvchat-net \
  -v mvchat_mvchat-uploads:/app/uploads \
  -p 6062:6062 --env-file .env tracker2api
```

## Project Structure

```
Tracker2API/
├── cmd/server/
│   └── main.go              # Entry point, router, CORS
├── internal/
│   ├── api/
│   │   ├── api.go           # HTTP handlers (~1700 lines)
│   │   └── invite.go        # Invite code handlers (~100 lines)
│   ├── auth/
│   │   └── auth.go          # Token validation (~94 lines)
│   ├── db/
│   │   └── db.go            # Database operations (~792 lines)
│   └── models/
│       └── models.go        # Structs & DTOs (~351 lines)
├── migrations/              # 6 SQL schema files
├── Dockerfile
├── docker-compose.yml
└── SERVER_SETUP.md          # Deployment guide
```

## Environment Variables

### Required
```bash
DATABASE_URL=postgresql://user:pass@host:5432/mvchat?sslmode=disable
AUTH_TOKEN_KEY=<base64-encoded-hmac-key>  # Must match mvchat2 TOKEN_KEY
```

### Optional
```bash
PORT=6062                    # Default: 8080
UPLOAD_PATH=/app/uploads     # File storage path
```

## API Endpoints

All endpoints require `Authorization: Bearer <token>` except `/health`.

### Health
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (no auth) |

### Pregnancy Management
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/pregnancy` | Get user's pregnancy (legacy) |
| POST | `/api/pregnancy` | Create new pregnancy |
| PUT | `/api/pregnancy` | Update pregnancy |
| GET | `/api/pregnancies` | List all accessible pregnancies |
| GET | `/api/pregnancies/{id}` | Get pregnancy by ID |
| PUT | `/api/pregnancies/{id}` | Update pregnancy by ID |
| GET | `/api/pregnancies/{id}/entries` | Get all entries for pregnancy |
| PUT | `/api/pregnancies/{id}/outcome` | Set pregnancy outcome |
| PUT | `/api/pregnancies/{id}/archive` | Archive/unarchive pregnancy |

### Entries
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/entries` | Get entries (query: type, since, includeDeleted) |
| POST | `/api/entries` | Create single entry |
| POST | `/api/entries/batch` | Create multiple entries |
| DELETE | `/api/entries/{clientId}` | Soft delete entry |

### Settings
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/settings` | Get all settings |
| PUT | `/api/settings/{type}` | Update setting |

### Sync
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/sync` | Pull all data since last sync |
| POST | `/api/sync` | Push local changes |

### Sharing / Invite Codes
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/sharing/status` | Get partner, supporters, active codes |
| POST | `/api/sharing/generate` | Generate invite code |
| POST | `/api/sharing/redeem` | Redeem invite code |
| POST | `/api/sharing/codes/{id}/revoke` | Revoke code |
| DELETE | `/api/sharing/supporters/{id}` | Remove supporter |
| GET | `/api/me/role` | Get user's role and permission |

### Legacy Pairing
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/pairing/request` | Create pairing request |
| GET | `/api/pairing/pending` | Get pending requests |
| POST | `/api/pairing/approve/{id}` | Approve request |
| POST | `/api/pairing/deny/{id}` | Deny request |
| PUT | `/api/pairing/permission` | Update partner permission |
| DELETE | `/api/pairing` | Remove pairing |
| GET | `/api/pairing/status` | Get pairing status |

### Files
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/files/upload` | Upload file (max 10MB) |
| GET | `/api/files/{id}` | Get file metadata |
| DELETE | `/api/files/{id}` | Soft delete file |

## Database Schema

All tables prefixed with `tracker2_` in shared `mvchat` database.

### tracker2_pregnancies
```sql
id BIGSERIAL PRIMARY KEY
owner_id BIGINT UNIQUE NOT NULL      -- Mother's user ID
partner_id BIGINT                    -- Father's user ID
partner_status VARCHAR(20)           -- pending/approved/denied
partner_permission VARCHAR(20)       -- read/write
partner_name VARCHAR(100)
display_partner_card BOOLEAN         -- Hide from UI if false

-- Pregnancy Data
due_date DATE
start_date DATE
calculation_method VARCHAR(20)       -- lmp/conception/due_date
cycle_length INT DEFAULT 28
baby_name VARCHAR(100)
mom_name VARCHAR(100)
mom_birthday DATE                    -- For age tracking
gender VARCHAR(20)                   -- boy/girl/unsure
parent_role VARCHAR(20)              -- mother/father
profile_photo TEXT

-- Outcomes
outcome VARCHAR(20) DEFAULT 'ongoing' -- ongoing/birth/miscarriage/ectopic/stillbirth
outcome_date DATE
archived BOOLEAN DEFAULT FALSE
archived_at TIMESTAMPTZ

created_at, updated_at TIMESTAMPTZ
```

### tracker2_entries
```sql
id BIGSERIAL PRIMARY KEY
pregnancy_id BIGINT NOT NULL REFERENCES tracker2_pregnancies
client_id VARCHAR(50) NOT NULL       -- Client-generated ID
entry_type VARCHAR(50) NOT NULL      -- weight/symptom/appointment/etc.
data JSONB NOT NULL                  -- Entry payload
created_at, updated_at TIMESTAMPTZ
deleted_at TIMESTAMPTZ               -- Soft delete

UNIQUE(pregnancy_id, entry_type, client_id)
```

**Entry Types:** weight, symptom, appointment, journal, water, photo, medical, intimacy, baby_name, kick_session, contraction_session

### tracker2_invite_codes
```sql
id BIGSERIAL PRIMARY KEY
pregnancy_id BIGINT NOT NULL REFERENCES tracker2_pregnancies
code_hash VARCHAR(60) NOT NULL       -- bcrypt hash
code_prefix VARCHAR(4) NOT NULL      -- First 4 chars for display
role VARCHAR(20) NOT NULL            -- father/support
permission VARCHAR(20) DEFAULT 'read'
expires_at TIMESTAMPTZ NOT NULL      -- NOW() + 48 hours
redeemed_at TIMESTAMPTZ
redeemed_by BIGINT
revoked_at TIMESTAMPTZ
```

### tracker2_supporters
```sql
id BIGSERIAL PRIMARY KEY
pregnancy_id BIGINT NOT NULL REFERENCES tracker2_pregnancies
user_id BIGINT NOT NULL
display_name VARCHAR(100)
display_partner_card BOOLEAN DEFAULT TRUE
joined_at TIMESTAMPTZ
invited_via_code_id BIGINT REFERENCES tracker2_invite_codes
removed_at TIMESTAMPTZ               -- Soft delete

UNIQUE(pregnancy_id, user_id)
```

### Other Tables
- `tracker2_settings` - User settings (JSONB data by type)
- `tracker2_files` - File metadata with storage_path
- `tracker2_pairing_requests` - Legacy partner requests
- `tracker2_sync_state` - Per-device sync tracking
- `tracker2_code_attempts` - Rate limiting for code redemption

## Authentication

### Token Format (JWT)
Standard JWT token with HMAC-SHA256 signing:
```json
{
  "uid": "<uuid>",    // User ID (UUID string)
  "iss": "mvchat2",
  "exp": 1234567890,  // Expiration timestamp
  "iat": 1234567890   // Issued at timestamp
}
```

### Validation Flow
1. Extract JWT from `Authorization: Bearer <token>`
2. Verify HMAC-SHA256 signature using TOKEN_KEY
3. Validate `exp` claim (not expired)
4. Extract `uid` claim as user ID
5. Store `UserInfo` in request context

`AUTH_TOKEN_KEY` must match mvchat2's `TOKEN_KEY` exactly (base64 encoded).

## Permission Model

### User Roles
| Role | Access | How Assigned |
|------|--------|--------------|
| `owner` | Full access | Creates pregnancy |
| `father` | Read or write | Redeems father invite code |
| `support` | Read only | Redeems support invite code |

### Permission Checks
```go
// Get user from context
user := getUserInfo(r)

// Check access with permission level
pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
if permission != "write" {
    writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
    return
}
```

### Admin Override
Email `tsrlegends@gmail.com` automatically gets write permission when redeeming any code. `displayPartnerCard: false` hides admin from UI.

## Invite Code System

### Code Format
- Pattern: `XXXX-XXXX-XX` (10 characters)
- Characters: 2-9, A-Z (excludes 0, O, 1, I, L)
- Storage: bcrypt hashed (not reversible)
- Display: prefix only (XXXX-****-**)
- Expiration: 48 hours

### Redemption Flow
1. User enters code
2. Server checks rate limit (5 failed/hour)
3. Iterate active codes, bcrypt.Compare each
4. If match found and not expired:
   - `father` role → set as partner on pregnancy
   - `support` role → create supporter record
5. Mark code as redeemed

## Error Responses

```json
{"error": {"code": "ERROR_CODE", "message": "Human readable message"}}
```

| Code | HTTP | Description |
|------|------|-------------|
| UNAUTHORIZED | 401 | Missing/invalid token |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource not found |
| CONFLICT | 409 | Business logic conflict |
| VALIDATION_ERROR | 400 | Invalid request |
| RATE_LIMITED | 429 | Too many attempts |
| INTERNAL_ERROR | 500 | Server error |

## Key Patterns

### Nullable Fields
```go
// Database: sql.Null* types
if p.PartnerID.Valid {
    partnerID := p.PartnerID.Int64
}

// DTOs: pointer types with omitempty
type PregnancyDTO struct {
    PartnerID *string `json:"partnerId,omitempty"`
}
```

### ID Formatting
```go
// User IDs are UUID strings from mvchat2
ownerId := pregnancy.OwnerID  // e.g., "fa497802-ba40-4447-bc48-6da2bf726926"
```

### UPSERT Pattern
Entries use `ON CONFLICT (pregnancy_id, entry_type, client_id) DO UPDATE` for idempotent creates.

## Migrations

| File | Description |
|------|-------------|
| 001_initial.sql | Core tables (pregnancies, entries, settings, files, sync_state) |
| 002_rename_fields.sql | Column renames (baby_gender→gender, etc.) |
| 002_pregnancy_outcomes.sql | Add outcome, outcome_date, archived fields |
| 003_invite_codes.sql | Invite codes, supporters, code_attempts tables |
| 004_display_partner_card.sql | Add displayPartnerCard to pregnancies/supporters |
| 005_mom_birthday.sql | Add mom_birthday for age tracking |
| 006_uuid_user_ids.sql | Convert user ID columns from BIGINT to TEXT for UUID support |

## Deployment

### Server
- Host: `root@scalecode.dev`
- Path: `/srv/docker/Tracker2API/`
- Domain: `api.clingy.me`
- Port: 6062

### Docker Network
- Network: `mvchat_mvchat-net`
- Database: `mvchat-db` (172.19.0.2:5432)
- Uploads: `mvchat_mvchat-uploads` volume

### Caddy Config
```
api.clingy.me {
    reverse_proxy localhost:6062
    # Rate limit: 100 req/min
}
```

## Related Projects

- **Clingy** (`/Users/tmarq/Github/Clingy`) - React Native client
- **mvchat** (`/Users/tmarq/Github/mvchat`) - Auth server
