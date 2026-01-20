# Tracker2API Migration Questions

**Source:** Copied from ClingyAPI
**Target Domain:** api.clingy.me
**Status:** Planning

---

## 1. Rename References

Current references to "clingy" that need updating:

| File/Location | Current | New |
|---------------|---------|-----|
| `go.mod` | `github.com/scalecode-solutions/clingyapi` | `github.com/scalecode-solutions/tracker2api`? |
| Internal imports | `clingyapi/internal/...` | `tracker2api/internal/...`? |
| Database tables | `clingy_pregnancies`, `clingy_entries`, etc. | `tracker2_*`? or keep `clingy_*`? |
| CLAUDE.md | ClingyAPI references | Tracker2API |
| Docker configs | clingyapi container name | tracker2api |

**Decision needed:**
- [ ] New Go module name
- [ ] New database table prefix (or keep existing?)
- [ ] New Docker container name

---

## 2. Database Strategy

**Option A: Same database, new table prefix**
- Use existing `mvchat` PostgreSQL instance
- Create new tables with `tracker2_` prefix
- Pros: Simple, shared infrastructure
- Cons: Tied to mvchat

**Option B: Same database, same tables**
- Reuse existing `clingy_*` tables
- Tracker2API replaces ClingyAPI
- Pros: No migration needed
- Cons: Can't run both simultaneously

**Option C: Separate database**
- New PostgreSQL database for Tracker2
- Pros: Complete isolation
- Cons: More infrastructure

**Decision needed:**
- [ ] Which database strategy?

---

## 3. New Data Endpoints

Add endpoints for baby sizes and weekly facts:

```
GET /api/data/baby-sizes
GET /api/data/weekly-facts
```

**Option A: Static JSON files**
- Store JSON files in API codebase
- Serve with `http.ServeFile()`
- Update by editing files and redeploying
- Pros: Simple
- Cons: Requires deploy to update

**Option B: Database tables**
- Create `tracker2_baby_sizes` and `tracker2_weekly_facts` tables
- Serve from database
- Pros: Can update via SQL/admin
- Cons: More setup

**Option C: Hybrid**
- JSON files with cache headers
- iOS caches locally
- Pros: Best of both
- Cons: Slightly more complex

**Decision needed:**
- [ ] Static files or database for baby sizes/weekly facts?

---

## 4. Features to Keep

Current ClingyAPI features - confirm which to keep:

| Feature | Keep? | Notes |
|---------|-------|-------|
| Pregnancy CRUD | ✅ Yes | Core functionality |
| Partner system | ✅ Yes | Father role with permissions |
| Supporter system | ✅ Yes | Family/friends read access |
| Invite codes | ✅ Yes | Sharing mechanism |
| Entries (weight, symptoms, kicks) | ✅ Yes | Tracking features |
| File uploads | ✅ Yes | Photos, etc. |
| Sync endpoints | ✅ Yes | Multi-device sync |
| Legacy pairing endpoints | ❓ | Still needed or remove? |
| Settings storage | ✅ Yes | User preferences |

**Decision needed:**
- [ ] Remove legacy pairing endpoints?
- [ ] Any other features to remove?

---

## 5. New Features to Add

Potential new features for Tracker2API:

| Feature | Priority | Notes |
|---------|----------|-------|
| Baby sizes endpoint | High | For iOS to fetch size data |
| Weekly facts endpoint | High | For iOS to fetch weekly content |
| Server-calculated progress | Low | Keep calculations on iOS? |
| Push notification triggers | Medium | Milestone notifications |
| Versioned data endpoints | Low | API versioning for data |

**Decision needed:**
- [ ] Which new features to implement?
- [ ] Keep calculations on iOS or add server-side?

---

## 6. Deployment Configuration

**Confirmed:**
- Domain: `api.clingy.me`

**To decide:**

| Setting | Current (ClingyAPI) | New (Tracker2API) |
|---------|---------------------|-------------------|
| Domain | clingy.mvchat.app | api.clingy.me |
| Port | 6062 | ? |
| Docker network | mvchat_mvchat-net | Same or new? |
| Database host | mvchat-db | Same or new? |
| Uploads volume | mvchat_mvchat-uploads | Same or new? |

**Decision needed:**
- [ ] Port number
- [ ] Docker network (same or separate?)
- [ ] Uploads storage location

---

## 7. Authentication

Current: Uses mvchat2 JWT tokens (shared AUTH_TOKEN_KEY)

**Options:**
- Keep using mvchat2 auth (same user accounts)
- Separate auth system for Tracker2

**Decision needed:**
- [ ] Keep mvchat2 auth or separate?

---

## 8. Migration Path

If existing ClingyAPI users need to migrate:

**Option A: No migration (fresh start)**
- Tracker2API is for new Clingy3 app only
- ClingyAPI continues for old Clingy app

**Option B: Data migration**
- Import existing pregnancy data to new tables
- Write migration script

**Decision needed:**
- [ ] Support migration from ClingyAPI?

---

## Summary Checklist

### Must Decide Before Development
- [ ] Database strategy (same tables, new prefix, or separate DB?)
- [ ] Authentication (mvchat2 or separate?)
- [ ] Port number

### Can Decide Later
- [ ] Baby sizes/weekly facts storage method
- [ ] New features beyond core functionality
- [ ] Legacy endpoint removal
- [ ] Migration path for existing users

---

## Next Steps

1. Answer critical questions above
2. Update go.mod and imports
3. Update/create database migrations
4. Add baby sizes and weekly facts endpoints
5. Update CLAUDE.md with new documentation
6. Test locally
7. Deploy to api.clingy.me
