// Package db provides database operations for Tracker2API.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/scalecode-solutions/tracker2api/internal/models"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// DB wraps database operations.
type DB struct {
	db *sqlx.DB
}

// New creates a new database connection.
func New(databaseURL string) (*DB, error) {
	db, err := sqlx.Connect("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// User operations

// GetUserEmail gets the user's email by user ID.
// In mvchat2, we query the users table by UUID
func (d *DB) GetUserEmail(ctx context.Context, userID string) (string, error) {
	var email sql.NullString
	err := d.db.GetContext(ctx, &email, `
		SELECT public->>'fn' FROM users WHERE id = $1
	`, userID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !email.Valid {
		return "", nil
	}
	return email.String, nil
}

// Pregnancy operations

// GetPregnancyByOwner gets pregnancy by owner ID.
func (d *DB) GetPregnancyByOwner(ctx context.Context, ownerID string) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.GetContext(ctx, &p, `
		SELECT * FROM clingy_pregnancies WHERE owner_id = $1
	`, ownerID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPregnancyByPartner gets pregnancy where user is the partner.
func (d *DB) GetPregnancyByPartner(ctx context.Context, partnerID string) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.GetContext(ctx, &p, `
		SELECT * FROM clingy_pregnancies
		WHERE partner_id = $1 AND partner_status = 'approved'
	`, partnerID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPregnancyByID gets pregnancy by ID.
func (d *DB) GetPregnancyByID(ctx context.Context, id int64) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.GetContext(ctx, &p, `SELECT * FROM clingy_pregnancies WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPregnanciesByUser gets all pregnancies a user has access to (owned + partner).
func (d *DB) ListPregnanciesByUser(ctx context.Context, userID string) ([]models.Pregnancy, error) {
	var pregnancies []models.Pregnancy
	err := d.db.SelectContext(ctx, &pregnancies, `
		SELECT * FROM clingy_pregnancies
		WHERE owner_id = $1
		   OR (partner_id = $1 AND partner_status = 'approved')
		ORDER BY archived ASC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	return pregnancies, nil
}

// SetPregnancyOutcome updates the outcome of a pregnancy.
func (d *DB) SetPregnancyOutcome(ctx context.Context, id int64, outcome string, outcomeDate *string) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.QueryRowxContext(ctx, `
		UPDATE clingy_pregnancies SET
			outcome = $2,
			outcome_date = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING *
	`, id, outcome, outcomeDate).StructScan(&p)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SetPregnancyArchive sets the archived status of a pregnancy.
func (d *DB) SetPregnancyArchive(ctx context.Context, id int64, archived bool) (*models.Pregnancy, error) {
	var p models.Pregnancy
	var err error
	if archived {
		err = d.db.QueryRowxContext(ctx, `
			UPDATE clingy_pregnancies SET
				archived = true,
				archived_at = NOW(),
				updated_at = NOW()
			WHERE id = $1
			RETURNING *
		`, id).StructScan(&p)
	} else {
		err = d.db.QueryRowxContext(ctx, `
			UPDATE clingy_pregnancies SET
				archived = false,
				archived_at = NULL,
				updated_at = NOW()
			WHERE id = $1
			RETURNING *
		`, id).StructScan(&p)
	}
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CreatePregnancy creates a new pregnancy record.
func (d *DB) CreatePregnancy(ctx context.Context, ownerID string, req *models.PregnancyRequest) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.QueryRowxContext(ctx, `
		INSERT INTO clingy_pregnancies (owner_id, due_date, start_date, calculation_method, cycle_length, baby_name, mom_name, mom_birthday, gender, parent_role)
		VALUES ($1, $2, $3, $4, COALESCE($5, 28), $6, $7, $8, $9, $10)
		RETURNING *
	`, ownerID, req.DueDate, req.StartDate, req.CalculationMethod, req.CycleLength, req.BabyName, req.MomName, req.MomBirthday, req.Gender, req.ParentRole).StructScan(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePregnancy updates an existing pregnancy record.
func (d *DB) UpdatePregnancy(ctx context.Context, id int64, req *models.PregnancyRequest) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.QueryRowxContext(ctx, `
		UPDATE clingy_pregnancies SET
			due_date = COALESCE($2, due_date),
			start_date = COALESCE($3, start_date),
			calculation_method = COALESCE($4, calculation_method),
			cycle_length = COALESCE($5, cycle_length),
			baby_name = COALESCE($6, baby_name),
			mom_name = COALESCE($7, mom_name),
			mom_birthday = COALESCE($8, mom_birthday),
			gender = COALESCE($9, gender),
			parent_role = COALESCE($10, parent_role),
			updated_at = NOW()
		WHERE id = $1
		RETURNING *
	`, id, req.DueDate, req.StartDate, req.CalculationMethod, req.CycleLength, req.BabyName, req.MomName, req.MomBirthday, req.Gender, req.ParentRole).StructScan(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Entry operations

// GetEntries gets entries for a pregnancy.
func (d *DB) GetEntries(ctx context.Context, pregnancyID int64, entryType string, since *time.Time, includeDeleted bool) ([]models.Entry, error) {
	query := `SELECT * FROM clingy_entries WHERE pregnancy_id = $1`
	args := []interface{}{pregnancyID}
	argNum := 2

	if entryType != "" {
		query += fmt.Sprintf(" AND entry_type = $%d", argNum)
		args = append(args, entryType)
		argNum++
	}

	if since != nil {
		query += fmt.Sprintf(" AND updated_at > $%d", argNum)
		args = append(args, since)
		argNum++
	}

	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}

	query += " ORDER BY created_at DESC"

	var entries []models.Entry
	err := d.db.SelectContext(ctx, &entries, query, args...)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// UpsertEntry creates or updates an entry.
func (d *DB) UpsertEntry(ctx context.Context, pregnancyID int64, req *models.EntryRequest) (*models.Entry, error) {
	var e models.Entry
	err := d.db.QueryRowxContext(ctx, `
		INSERT INTO clingy_entries (pregnancy_id, client_id, entry_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (pregnancy_id, entry_type, client_id) DO UPDATE SET
			data = EXCLUDED.data,
			updated_at = NOW(),
			deleted_at = NULL
		RETURNING *
	`, pregnancyID, req.ClientID, req.EntryType, req.Data).StructScan(&e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// DeleteEntry soft deletes an entry.
func (d *DB) DeleteEntry(ctx context.Context, pregnancyID int64, clientID string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_entries SET deleted_at = NOW(), updated_at = NOW()
		WHERE pregnancy_id = $1 AND client_id = $2 AND deleted_at IS NULL
	`, pregnancyID, clientID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Settings operations

// GetSettings gets all settings for a pregnancy.
func (d *DB) GetSettings(ctx context.Context, pregnancyID int64) (map[string]json.RawMessage, error) {
	var settings []models.Setting
	err := d.db.SelectContext(ctx, &settings, `
		SELECT * FROM clingy_settings WHERE pregnancy_id = $1
	`, pregnancyID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]json.RawMessage)
	for _, s := range settings {
		result[s.SettingType] = s.Data
	}
	return result, nil
}

// UpsertSetting creates or updates a setting.
func (d *DB) UpsertSetting(ctx context.Context, pregnancyID int64, settingType string, data json.RawMessage) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO clingy_settings (pregnancy_id, setting_type, data)
		VALUES ($1, $2, $3)
		ON CONFLICT (pregnancy_id, setting_type) DO UPDATE SET
			data = EXCLUDED.data,
			updated_at = NOW()
	`, pregnancyID, settingType, data)
	return err
}

// Pairing operations

// CreatePairingRequest creates a new pairing request.
func (d *DB) CreatePairingRequest(ctx context.Context, requesterID string, requesterName, targetEmail string) (*models.PairingRequest, error) {
	// First try to find the target user by email
	var targetID sql.NullString
	err := d.db.GetContext(ctx, &targetID, `
		SELECT id FROM users WHERE LOWER(tags->>'email') = LOWER($1)
	`, targetEmail)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var pr models.PairingRequest
	err = d.db.QueryRowxContext(ctx, `
		INSERT INTO clingy_pairing_requests (requester_id, requester_name, target_email, target_id, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING *
	`, requesterID, requesterName, targetEmail, targetID).StructScan(&pr)
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

// GetPendingPairingRequests gets pending requests for a user.
func (d *DB) GetPendingPairingRequests(ctx context.Context, targetID string) ([]models.PairingRequest, error) {
	var requests []models.PairingRequest
	err := d.db.SelectContext(ctx, &requests, `
		SELECT * FROM clingy_pairing_requests
		WHERE target_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`, targetID)
	if err != nil {
		return nil, err
	}
	return requests, nil
}

// ApprovePairingRequest approves a pairing request.
func (d *DB) ApprovePairingRequest(ctx context.Context, requestID int64, targetID string, permission string) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the request
	var pr models.PairingRequest
	err = tx.GetContext(ctx, &pr, `
		SELECT * FROM clingy_pairing_requests WHERE id = $1 AND target_id = $2 AND status = 'pending'
	`, requestID, targetID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Update the request
	_, err = tx.ExecContext(ctx, `
		UPDATE clingy_pairing_requests SET status = 'approved', permission = $1, resolved_at = NOW()
		WHERE id = $2
	`, permission, requestID)
	if err != nil {
		return err
	}

	// Update the pregnancy
	_, err = tx.ExecContext(ctx, `
		UPDATE clingy_pregnancies SET
			partner_id = $1,
			partner_status = 'approved',
			partner_permission = $2,
			updated_at = NOW()
		WHERE owner_id = $3
	`, pr.RequesterID, permission, targetID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DenyPairingRequest denies a pairing request.
func (d *DB) DenyPairingRequest(ctx context.Context, requestID int64, targetID string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_pairing_requests SET status = 'denied', resolved_at = NOW()
		WHERE id = $1 AND target_id = $2 AND status = 'pending'
	`, requestID, targetID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePartnerPermission updates partner's permission level.
func (d *DB) UpdatePartnerPermission(ctx context.Context, ownerID string, permission string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_pregnancies SET partner_permission = $1, updated_at = NOW()
		WHERE owner_id = $2 AND partner_id IS NOT NULL
	`, permission, ownerID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// RemovePairing removes a pairing.
func (d *DB) RemovePairing(ctx context.Context, userID string) error {
	// Try as owner first
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_pregnancies SET
			partner_id = NULL,
			partner_status = NULL,
			partner_permission = NULL,
			updated_at = NOW()
		WHERE owner_id = $1 AND partner_id IS NOT NULL
	`, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	// Try as partner
	result, err = d.db.ExecContext(ctx, `
		UPDATE clingy_pregnancies SET
			partner_id = NULL,
			partner_status = NULL,
			partner_permission = NULL,
			updated_at = NOW()
		WHERE partner_id = $1
	`, userID)
	if err != nil {
		return err
	}
	rows, _ = result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// File operations

// CreateFile creates a file record.
func (d *DB) CreateFile(ctx context.Context, pregnancyID int64, file *models.File) (*models.File, error) {
	var f models.File
	err := d.db.QueryRowxContext(ctx, `
		INSERT INTO clingy_files (pregnancy_id, client_id, file_type, storage_path, mime_type, size_bytes, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING *
	`, pregnancyID, file.ClientID, file.FileType, file.StoragePath, file.MimeType, file.SizeBytes, file.Metadata).StructScan(&f)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetFile gets a file by ID.
func (d *DB) GetFile(ctx context.Context, fileID int64) (*models.File, error) {
	var f models.File
	err := d.db.GetContext(ctx, &f, `
		SELECT * FROM clingy_files WHERE id = $1 AND deleted_at IS NULL
	`, fileID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteFile soft deletes a file.
func (d *DB) DeleteFile(ctx context.Context, fileID int64) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_files SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, fileID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Sync operations

// GetSyncState gets sync state for a device.
func (d *DB) GetSyncState(ctx context.Context, userID string, deviceID string) (*models.SyncState, error) {
	var ss models.SyncState
	err := d.db.GetContext(ctx, &ss, `
		SELECT * FROM clingy_sync_state WHERE user_id = $1 AND device_id = $2
	`, userID, deviceID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

// UpdateSyncState updates sync state for a device.
func (d *DB) UpdateSyncState(ctx context.Context, userID string, deviceID string, syncVersion int64) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO clingy_sync_state (user_id, device_id, last_sync_at, last_sync_version)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			last_sync_at = NOW(),
			last_sync_version = EXCLUDED.last_sync_version
	`, userID, deviceID, syncVersion)
	return err
}

// ============ Invite Code Operations ============

// CreateInviteCode creates a new invite code record.
func (d *DB) CreateInviteCode(ctx context.Context, pregnancyID int64, codeHash, codePrefix, role, permission string, expiresAt time.Time) (*models.InviteCode, error) {
	var code models.InviteCode
	err := d.db.QueryRowxContext(ctx, `
		INSERT INTO clingy_invite_codes (pregnancy_id, code_hash, code_prefix, role, permission, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING *
	`, pregnancyID, codeHash, codePrefix, role, permission, expiresAt).StructScan(&code)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// GetActiveInviteCodes gets all active (non-redeemed, non-revoked, non-expired) codes for a pregnancy.
func (d *DB) GetActiveInviteCodes(ctx context.Context, pregnancyID int64) ([]models.InviteCode, error) {
	var codes []models.InviteCode
	err := d.db.SelectContext(ctx, &codes, `
		SELECT * FROM clingy_invite_codes
		WHERE pregnancy_id = $1
		  AND redeemed_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		ORDER BY created_at DESC
	`, pregnancyID)
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// FindValidInviteCode finds an active invite code by hash verification.
// Returns all active codes for iteration (caller must verify hash).
func (d *DB) FindActiveInviteCodes(ctx context.Context) ([]models.InviteCode, error) {
	var codes []models.InviteCode
	err := d.db.SelectContext(ctx, &codes, `
		SELECT * FROM clingy_invite_codes
		WHERE redeemed_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`)
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// Admin email that gets full write access regardless of role
const adminEmail = "tsrlegends@gmail.com"

// RedeemInviteCode marks a code as redeemed and returns the associated pregnancy.
// If email matches admin email, permission is upgraded to 'write'.
func (d *DB) RedeemInviteCode(ctx context.Context, codeID int64, userID string, displayName, email string) (*models.Pregnancy, string, error) {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	// Get and lock the invite code
	var code models.InviteCode
	err = tx.GetContext(ctx, &code, `
		SELECT * FROM clingy_invite_codes
		WHERE id = $1 AND redeemed_at IS NULL AND revoked_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, codeID)
	if err == sql.ErrNoRows {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}

	// Mark code as redeemed
	_, err = tx.ExecContext(ctx, `
		UPDATE clingy_invite_codes SET redeemed_at = NOW(), redeemed_by = $1
		WHERE id = $2
	`, userID, codeID)
	if err != nil {
		return nil, "", err
	}

	// Determine permission - admin email gets write access
	permission := code.Permission
	isAdmin := email == adminEmail
	if isAdmin {
		permission = "write"
	}

	// Handle based on role
	if code.Role == "father" {
		// Update pregnancy with partner info
		// Admin email doesn't show in partner card UI
		_, err = tx.ExecContext(ctx, `
			UPDATE clingy_pregnancies SET
				partner_id = $1,
				partner_status = 'approved',
				partner_permission = $2,
				partner_name = $3,
				display_partner_card = $5,
				updated_at = NOW()
			WHERE id = $4
		`, userID, permission, displayName, code.PregnancyID, !isAdmin)
		if err != nil {
			return nil, "", err
		}
	} else {
		// Create supporter record
		// Admin email doesn't show in partner card UI
		_, err = tx.ExecContext(ctx, `
			INSERT INTO clingy_supporters (pregnancy_id, user_id, display_name, invited_via_code_id, display_partner_card)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (pregnancy_id, user_id) DO UPDATE SET
				display_name = EXCLUDED.display_name,
				removed_at = NULL,
				joined_at = NOW(),
				display_partner_card = EXCLUDED.display_partner_card
		`, code.PregnancyID, userID, displayName, codeID, !isAdmin)
		if err != nil {
			return nil, "", err
		}
	}

	// Get pregnancy
	var pregnancy models.Pregnancy
	err = tx.GetContext(ctx, &pregnancy, `SELECT * FROM clingy_pregnancies WHERE id = $1`, code.PregnancyID)
	if err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	return &pregnancy, permission, nil
}

// RevokeInviteCode revokes an invite code.
func (d *DB) RevokeInviteCode(ctx context.Context, codeID int64, ownerID string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_invite_codes SET revoked_at = NOW()
		WHERE id = $1
		  AND pregnancy_id IN (SELECT id FROM clingy_pregnancies WHERE owner_id = $2)
		  AND redeemed_at IS NULL
		  AND revoked_at IS NULL
	`, codeID, ownerID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetInviteCodeByID gets an invite code by ID.
func (d *DB) GetInviteCodeByID(ctx context.Context, codeID int64) (*models.InviteCode, error) {
	var code models.InviteCode
	err := d.db.GetContext(ctx, &code, `SELECT * FROM clingy_invite_codes WHERE id = $1`, codeID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// ============ Supporter Operations ============

// GetSupporters gets all active supporters for a pregnancy.
func (d *DB) GetSupporters(ctx context.Context, pregnancyID int64) ([]models.Supporter, error) {
	var supporters []models.Supporter
	err := d.db.SelectContext(ctx, &supporters, `
		SELECT * FROM clingy_supporters
		WHERE pregnancy_id = $1 AND removed_at IS NULL
		ORDER BY joined_at DESC
	`, pregnancyID)
	if err != nil {
		return nil, err
	}
	return supporters, nil
}

// GetPregnancyBySupporter gets pregnancy where user is a supporter.
func (d *DB) GetPregnancyBySupporter(ctx context.Context, userID string) (*models.Pregnancy, error) {
	var p models.Pregnancy
	err := d.db.GetContext(ctx, &p, `
		SELECT p.* FROM clingy_pregnancies p
		JOIN clingy_supporters s ON s.pregnancy_id = p.id
		WHERE s.user_id = $1 AND s.removed_at IS NULL
	`, userID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// RemoveSupporter removes a supporter (soft delete).
func (d *DB) RemoveSupporter(ctx context.Context, supporterID int64, ownerID string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE clingy_supporters SET removed_at = NOW()
		WHERE id = $1
		  AND pregnancy_id IN (SELECT id FROM clingy_pregnancies WHERE owner_id = $2)
		  AND removed_at IS NULL
	`, supporterID, ownerID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ============ Rate Limiting Operations ============

// CountRecentCodeAttempts counts failed code attempts in the last hour.
func (d *DB) CountRecentCodeAttempts(ctx context.Context, userID string) (int, error) {
	var count int
	err := d.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM clingy_code_attempts
		WHERE user_id = $1 AND attempted_at > NOW() - INTERVAL '1 hour' AND success = false
	`, userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// RecordCodeAttempt records a code redemption attempt.
func (d *DB) RecordCodeAttempt(ctx context.Context, userID string, success bool, ipAddress string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO clingy_code_attempts (user_id, success, ip_address)
		VALUES ($1, $2, $3)
	`, userID, success, ipAddress)
	return err
}
