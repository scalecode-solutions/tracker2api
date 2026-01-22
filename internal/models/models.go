// Package models defines the data structures for Tracker2API.
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Pregnancy represents a pregnancy record.
type Pregnancy struct {
	ID                int64           `db:"id" json:"id"`
	OwnerID           string          `db:"owner_id" json:"ownerId"`
	PartnerID         sql.NullString  `db:"partner_id" json:"partnerId,omitempty"`
	PartnerStatus     sql.NullString  `db:"partner_status" json:"partnerStatus,omitempty"`
	PartnerPermission sql.NullString  `db:"partner_permission" json:"partnerPermission,omitempty"`
	PartnerName       sql.NullString  `db:"partner_name" json:"partnerName,omitempty"`
	DueDate           sql.NullTime    `db:"due_date" json:"dueDate,omitempty"`
	StartDate         sql.NullTime    `db:"start_date" json:"startDate,omitempty"`
	CalculationMethod sql.NullString  `db:"calculation_method" json:"calculationMethod,omitempty"`
	CycleLength       int             `db:"cycle_length" json:"cycleLength"`
	BabyName          sql.NullString  `db:"baby_name" json:"babyName,omitempty"`
	MomName           sql.NullString  `db:"mom_name" json:"momName,omitempty"`
	MomBirthday       sql.NullTime    `db:"mom_birthday" json:"momBirthday,omitempty"`
	Gender            sql.NullString  `db:"gender" json:"gender,omitempty"`
	ParentRole        sql.NullString  `db:"parent_role" json:"parentRole,omitempty"`
	ProfilePhoto       sql.NullString  `db:"profile_photo" json:"profilePhoto,omitempty"`
	DisplayPartnerCard sql.NullBool    `db:"display_partner_card" json:"displayPartnerCard,omitempty"`
	CoownerID          sql.NullString  `db:"coowner_id" json:"coownerId,omitempty"`
	CoownerName        sql.NullString  `db:"coowner_name" json:"coownerName,omitempty"`
	Outcome            sql.NullString  `db:"outcome" json:"outcome,omitempty"`
	OutcomeDate       sql.NullTime    `db:"outcome_date" json:"outcomeDate,omitempty"`
	Archived          bool            `db:"archived" json:"archived"`
	ArchivedAt        sql.NullTime    `db:"archived_at" json:"archivedAt,omitempty"`
	CreatedAt         time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt         time.Time       `db:"updated_at" json:"updatedAt"`
}

// Entry represents a generic entry record.
type Entry struct {
	ID          int64           `db:"id" json:"id"`
	PregnancyID int64           `db:"pregnancy_id" json:"-"`
	ClientID    string          `db:"client_id" json:"clientId"`
	EntryType   string          `db:"entry_type" json:"entryType"`
	Data        json.RawMessage `db:"data" json:"data"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updatedAt"`
	DeletedAt   sql.NullTime    `db:"deleted_at" json:"deletedAt,omitempty"`
}

// Setting represents a user setting.
type Setting struct {
	ID          int64           `db:"id" json:"id"`
	PregnancyID int64           `db:"pregnancy_id" json:"-"`
	SettingType string          `db:"setting_type" json:"settingType"`
	Data        json.RawMessage `db:"data" json:"data"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updatedAt"`
}

// PairingRequest represents a partner pairing request.
type PairingRequest struct {
	ID            int64          `db:"id" json:"id"`
	RequesterID   string         `db:"requester_id" json:"requesterId"`
	RequesterName sql.NullString `db:"requester_name" json:"requesterName,omitempty"`
	TargetEmail   string         `db:"target_email" json:"targetEmail"`
	TargetID      sql.NullString `db:"target_id" json:"targetId,omitempty"`
	Status        string         `db:"status" json:"status"`
	Permission    sql.NullString `db:"permission" json:"permission,omitempty"`
	CreatedAt     time.Time      `db:"created_at" json:"createdAt"`
	ResolvedAt    sql.NullTime   `db:"resolved_at" json:"resolvedAt,omitempty"`
}

// File represents an uploaded file.
type File struct {
	ID          int64           `db:"id" json:"id"`
	PregnancyID int64           `db:"pregnancy_id" json:"-"`
	ClientID    sql.NullString  `db:"client_id" json:"clientId,omitempty"`
	FileType    string          `db:"file_type" json:"fileType"`
	StoragePath string          `db:"storage_path" json:"storagePath"`
	MimeType    sql.NullString  `db:"mime_type" json:"mimeType,omitempty"`
	SizeBytes   sql.NullInt64   `db:"size_bytes" json:"sizeBytes,omitempty"`
	Metadata    json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
	DeletedAt   sql.NullTime    `db:"deleted_at" json:"deletedAt,omitempty"`
}

// SyncState represents sync state per device.
type SyncState struct {
	ID              int64        `db:"id" json:"id"`
	UserID          string       `db:"user_id" json:"userId"`
	DeviceID        string       `db:"device_id" json:"deviceId"`
	LastSyncAt      sql.NullTime `db:"last_sync_at" json:"lastSyncAt,omitempty"`
	LastSyncVersion int64        `db:"last_sync_version" json:"lastSyncVersion"`
}

// API Request/Response types

// PregnancyRequest is the request body for creating/updating pregnancy.
type PregnancyRequest struct {
	DueDate           *string `json:"dueDate,omitempty"`
	StartDate         *string `json:"startDate,omitempty"`
	CalculationMethod *string `json:"calculationMethod,omitempty"`
	CycleLength       *int    `json:"cycleLength,omitempty"`
	BabyName          *string `json:"babyName,omitempty"`
	MomName           *string `json:"momName,omitempty"`
	MomBirthday       *string `json:"momBirthday,omitempty"`
	Gender            *string `json:"gender,omitempty"`
	ParentRole        *string `json:"parentRole,omitempty"`
}

// PregnancyResponse is the response for pregnancy endpoints.
type PregnancyResponse struct {
	Pregnancy  *PregnancyDTO `json:"pregnancy"`
	Role       string        `json:"role"`
	Permission string        `json:"permission"`
}

// PregnancyDTO is the data transfer object for pregnancy.
type PregnancyDTO struct {
	ID                int64   `json:"id"`
	OwnerID           string  `json:"ownerId"`
	PartnerID         *string `json:"partnerId,omitempty"`
	PartnerPermission *string `json:"partnerPermission,omitempty"`
	DueDate           *string `json:"dueDate,omitempty"`
	StartDate         *string `json:"startDate,omitempty"`
	CalculationMethod *string `json:"calculationMethod,omitempty"`
	CycleLength       int     `json:"cycleLength"`
	BabyName          *string `json:"babyName,omitempty"`
	MomName           *string `json:"momName,omitempty"`
	MomBirthday       *string `json:"momBirthday,omitempty"`
	Gender            *string `json:"gender,omitempty"`
	ParentRole        *string `json:"parentRole,omitempty"`
	ProfilePhoto      *string `json:"profilePhoto,omitempty"`
	Outcome           *string `json:"outcome,omitempty"`
	OutcomeDate       *string `json:"outcomeDate,omitempty"`
	Archived          bool    `json:"archived"`
	ArchivedAt        *string `json:"archivedAt,omitempty"`
}

// EntryRequest is the request body for creating an entry.
type EntryRequest struct {
	ClientID  string          `json:"clientId"`
	EntryType string          `json:"entryType"`
	Data      json.RawMessage `json:"data"`
}

// BatchEntryRequest is the request body for batch creating entries.
type BatchEntryRequest struct {
	Entries []EntryRequest `json:"entries"`
}

// EntriesResponse is the response for entries endpoints.
type EntriesResponse struct {
	Entries     []Entry `json:"entries"`
	SyncVersion int64   `json:"syncVersion"`
}

// PairingRequestBody is the request body for creating a pairing request.
type PairingRequestBody struct {
	TargetEmail   string `json:"targetEmail"`
	RequesterName string `json:"requesterName"`
}

// ApprovalRequest is the request body for approving a pairing request.
type ApprovalRequest struct {
	Permission string `json:"permission"` // "read" or "write"
}

// PermissionRequest is the request body for updating partner permission.
type PermissionRequest struct {
	Permission string `json:"permission"` // "read" or "write"
}

// PairingStatusResponse is the response for pairing status.
type PairingStatusResponse struct {
	Paired  bool         `json:"paired"`
	Partner *PartnerInfo `json:"partner,omitempty"`
	Role    string       `json:"role"`
}

// PartnerInfo contains partner information.
type PartnerInfo struct {
	ID                 string `json:"id"`
	Name               string `json:"name,omitempty"`
	Permission         string `json:"permission"`
	PairedAt           string `json:"pairedAt"`
	DisplayPartnerCard bool   `json:"displayPartnerCard"`
}

// SyncRequest is the request body for posting sync data.
type SyncRequest struct {
	DeviceID        string           `json:"deviceId"`
	LastSyncVersion int64            `json:"lastSyncVersion"`
	Pregnancy       *PregnancyRequest `json:"pregnancy,omitempty"`
	Entries         []EntryRequest   `json:"entries,omitempty"`
	Settings        map[string]json.RawMessage `json:"settings,omitempty"`
	DeletedEntries  []string         `json:"deletedEntries,omitempty"`
}

// SyncResponse is the response for sync endpoints.
type SyncResponse struct {
	Pregnancy   *PregnancyDTO                 `json:"pregnancy,omitempty"`
	Entries     map[string][]Entry            `json:"entries,omitempty"`
	Settings    map[string]json.RawMessage    `json:"settings,omitempty"`
	Files       []File                        `json:"files,omitempty"`
	SyncVersion int64                         `json:"syncVersion"`
	ServerTime  string                        `json:"serverTime"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OutcomeRequest is the request body for setting pregnancy outcome.
type OutcomeRequest struct {
	Outcome     string  `json:"outcome"`
	OutcomeDate *string `json:"outcomeDate,omitempty"`
}

// ArchiveRequest is the request body for archiving/unarchiving a pregnancy.
type ArchiveRequest struct {
	Archived bool `json:"archived"`
}

// PregnancyWithRole includes pregnancy data with user's role and permission.
type PregnancyWithRole struct {
	Pregnancy  *PregnancyDTO `json:"pregnancy"`
	Role       string        `json:"role"`
	Permission string        `json:"permission"`
}

// PregnanciesResponse is the response for listing all pregnancies.
type PregnanciesResponse struct {
	Pregnancies []PregnancyWithRole `json:"pregnancies"`
}

// ============ Invite Code / Sharing Models ============

// UserRole represents the user's role for a pregnancy.
type UserRole string

const (
	UserRoleOwner   UserRole = "owner"
	UserRoleFather  UserRole = "father"
	UserRoleSupport UserRole = "support"
)

// InviteCode represents a sharing invite code.
type InviteCode struct {
	ID          int64          `db:"id" json:"id"`
	PregnancyID int64          `db:"pregnancy_id" json:"-"`
	CodeHash    string         `db:"code_hash" json:"-"`
	CodePrefix  string         `db:"code_prefix" json:"codePrefix"`
	Role        string         `db:"role" json:"role"`
	Permission  string         `db:"permission" json:"permission"`
	CreatedAt   time.Time      `db:"created_at" json:"createdAt"`
	ExpiresAt   time.Time      `db:"expires_at" json:"expiresAt"`
	RedeemedAt  sql.NullTime   `db:"redeemed_at" json:"redeemedAt,omitempty"`
	RedeemedBy  sql.NullString `db:"redeemed_by" json:"redeemedBy,omitempty"`
	RevokedAt   sql.NullTime   `db:"revoked_at" json:"revokedAt,omitempty"`
}

// Supporter represents a support user with limited access.
type Supporter struct {
	ID                 int64          `db:"id" json:"id"`
	PregnancyID        int64          `db:"pregnancy_id" json:"-"`
	UserID             string         `db:"user_id" json:"userId"`
	DisplayName        sql.NullString `db:"display_name" json:"displayName,omitempty"`
	Permission         sql.NullString `db:"permission" json:"permission,omitempty"`
	JoinedAt           time.Time      `db:"joined_at" json:"joinedAt"`
	InvitedViaCodeID   sql.NullInt64  `db:"invited_via_code_id" json:"-"`
	RemovedAt          sql.NullTime   `db:"removed_at" json:"removedAt,omitempty"`
	DisplayPartnerCard sql.NullBool   `db:"display_partner_card" json:"displayPartnerCard,omitempty"`
}

// CodeAttempt represents a code redemption attempt for rate limiting.
type CodeAttempt struct {
	ID          int64          `db:"id"`
	UserID      string         `db:"user_id"`
	AttemptedAt time.Time      `db:"attempted_at"`
	Success     bool           `db:"success"`
	IPAddress   sql.NullString `db:"ip_address"`
}

// GenerateCodeRequest is the request body for generating an invite code.
type GenerateCodeRequest struct {
	Role       string `json:"role"`                 // "father" or "support"
	Permission string `json:"permission,omitempty"` // "read" or "write" (default: read)
}

// GenerateCodeResponse is the response after generating a code.
type GenerateCodeResponse struct {
	Code      string    `json:"code"`      // Full code: XXXX-XXXX-XX
	ExpiresAt time.Time `json:"expiresAt"`
	Role      string    `json:"role"`
}

// RedeemCodeRequest is the request body for redeeming a code.
type RedeemCodeRequest struct {
	Code        string `json:"code"`        // Full code: XXXX-XXXX-XX
	DisplayName string `json:"displayName"` // User's display name
	Email       string `json:"email"`       // User's email (for admin check)
}

// RedeemCodeResponse is the response after redeeming a code.
type RedeemCodeResponse struct {
	Success    bool          `json:"success"`
	Role       string        `json:"role"`       // "father" or "support"
	Permission string        `json:"permission"` // "read" or "write"
	Pregnancy  *PregnancyDTO `json:"pregnancy"`  // Connected pregnancy info
	MomName    string        `json:"momName"`
	BabyName   string        `json:"babyName"`
	DueDate    string        `json:"dueDate,omitempty"`
}

// SupporterInfo contains supporter information for display.
type SupporterInfo struct {
	ID                 int64  `json:"id"`
	UserID             string `json:"userId"`
	DisplayName        string `json:"displayName"`
	JoinedAt           string `json:"joinedAt"`
	DisplayPartnerCard bool   `json:"displayPartnerCard"`
}

// ActiveCodeInfo contains active invite code information for display.
type ActiveCodeInfo struct {
	ID         int64  `json:"id"`
	CodePrefix string `json:"codePrefix"` // XXXX-****-**
	Role       string `json:"role"`
	ExpiresAt  string `json:"expiresAt"`
	ExpiresIn  string `json:"expiresIn"` // "23h 45m"
}

// SharingStatus is the response for sharing status endpoint.
type SharingStatus struct {
	Partner     *PartnerInfo     `json:"partner,omitempty"`
	Supporters  []SupporterInfo  `json:"supporters"`
	ActiveCodes []ActiveCodeInfo `json:"activeCodes"`
}

// MyRoleResponse is the response for the /api/me/role endpoint.
type MyRoleResponse struct {
	Role       string        `json:"role"`       // "owner", "father", "support", or "" if no access
	Permission string        `json:"permission"` // "read" or "write"
	Pregnancy  *PregnancyDTO `json:"pregnancy,omitempty"`
}
