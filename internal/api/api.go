// Package api provides HTTP handlers for Tracker2API.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/scalecode-solutions/tracker2api/internal/auth"
	"github.com/scalecode-solutions/tracker2api/internal/db"
	"github.com/scalecode-solutions/tracker2api/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"


// Handler provides HTTP handlers for the API.
type Handler struct {
	db         *db.DB
	auth       *auth.Authenticator
	uploadPath string
	dataPath   string
}

// New creates a new API handler.
func New(database *db.DB, authenticator *auth.Authenticator, uploadPath string, dataPath string) *Handler {
	return &Handler{
		db:         database,
		auth:       authenticator,
		uploadPath: uploadPath,
		dataPath:   dataPath,
	}
}

// AuthMiddleware validates JWT tokens.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format")
			return
		}

		// JWT tokens are passed as-is, no base64 decoding needed
		tokenString := parts[1]

		userInfo, err := h.auth.ValidateToken(tokenString)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, userInfo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getUserInfo(r *http.Request) *auth.UserInfo {
	return r.Context().Value(userContextKey).(*auth.UserInfo)
}

// Pregnancy endpoints

// GetPregnancy gets the current user's pregnancy or partner's pregnancy.
func (h *Handler) GetPregnancy(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	// First try to get pregnancy as owner
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, user.UserID)
	if err == nil {
		resp := models.PregnancyResponse{
			Pregnancy:  toPregnancyDTO(pregnancy),
			Role:       "owner",
			Permission: "write",
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if err != db.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Try as partner
	pregnancy, err = h.db.GetPregnancyByPartner(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	permission := "read"
	if pregnancy.PartnerPermission.Valid {
		permission = pregnancy.PartnerPermission.String
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(pregnancy),
		Role:       "partner",
		Permission: permission,
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreatePregnancy creates a new pregnancy record.
// Users can have multiple pregnancies (for tracking history).
func (h *Handler) CreatePregnancy(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	var req models.PregnancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	pregnancy, err := h.db.CreatePregnancy(ctx, user.UserID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(pregnancy),
		Role:       "owner",
		Permission: "write",
	}
	writeJSON(w, http.StatusCreated, resp)
}

// UpdatePregnancy updates the pregnancy record.
func (h *Handler) UpdatePregnancy(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	var req models.PregnancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	updated, err := h.db.UpdatePregnancy(ctx, pregnancy.ID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	role := "owner"
	if pregnancy.OwnerID != user.UserID {
		role = "partner"
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(updated),
		Role:       role,
		Permission: permission,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListPregnancies lists all pregnancies the user has access to.
func (h *Handler) ListPregnancies(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancies, err := h.db.ListPregnanciesByUser(ctx, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var result []models.PregnancyWithRole
	for _, p := range pregnancies {
		role := "owner"
		permission := "write"
		if p.OwnerID != user.UserID {
			role = "partner"
			if p.PartnerPermission.Valid {
				permission = p.PartnerPermission.String
			} else {
				permission = "read"
			}
		}
		pCopy := p // avoid closure issue
		result = append(result, models.PregnancyWithRole{
			Pregnancy:  toPregnancyDTO(&pCopy),
			Role:       role,
			Permission: permission,
		})
	}

	writeJSON(w, http.StatusOK, models.PregnanciesResponse{Pregnancies: result})
}

// GetPregnancyByID gets a specific pregnancy by ID.
func (h *Handler) GetPregnancyByID(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	pregnancyID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pregnancy ID")
		return
	}

	pregnancy, err := h.db.GetPregnancyByID(ctx, pregnancyID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pregnancy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Check access
	role := ""
	permission := ""
	if pregnancy.OwnerID == user.UserID {
		role = "owner"
		permission = "write"
	} else if pregnancy.PartnerID.Valid && pregnancy.PartnerID.String == user.UserID && pregnancy.PartnerStatus.String == "approved" {
		role = "partner"
		if pregnancy.PartnerPermission.Valid {
			permission = pregnancy.PartnerPermission.String
		} else {
			permission = "read"
		}
	} else {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
		return
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(pregnancy),
		Role:       role,
		Permission: permission,
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdatePregnancyByID updates a specific pregnancy by ID.
func (h *Handler) UpdatePregnancyByID(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	pregnancyID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pregnancy ID")
		return
	}

	pregnancy, err := h.db.GetPregnancyByID(ctx, pregnancyID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pregnancy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Check write access
	role := ""
	permission := ""
	if pregnancy.OwnerID == user.UserID {
		role = "owner"
		permission = "write"
	} else if pregnancy.PartnerID.Valid && pregnancy.PartnerID.String == user.UserID && pregnancy.PartnerStatus.String == "approved" {
		role = "partner"
		if pregnancy.PartnerPermission.Valid {
			permission = pregnancy.PartnerPermission.String
		} else {
			permission = "read"
		}
	} else {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	var req models.PregnancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	updated, err := h.db.UpdatePregnancy(ctx, pregnancyID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(updated),
		Role:       role,
		Permission: permission,
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetPregnancyEntries gets all entries for a specific pregnancy.
func (h *Handler) GetPregnancyEntries(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	pregnancyID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pregnancy ID")
		return
	}

	pregnancy, err := h.db.GetPregnancyByID(ctx, pregnancyID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pregnancy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Check access
	hasAccess := pregnancy.OwnerID == user.UserID ||
		(pregnancy.PartnerID.Valid && pregnancy.PartnerID.String == user.UserID && pregnancy.PartnerStatus.String == "approved")
	if !hasAccess {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
		return
	}

	entries, err := h.db.GetEntries(ctx, pregnancyID, "", nil, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Group by type
	entriesByType := make(map[string][]models.Entry)
	for _, e := range entries {
		entriesByType[e.EntryType] = append(entriesByType[e.EntryType], e)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries":     entriesByType,
		"syncVersion": time.Now().UnixMilli(),
	})
}

// SetPregnancyOutcome sets the outcome of a pregnancy.
func (h *Handler) SetPregnancyOutcome(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	pregnancyID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pregnancy ID")
		return
	}

	pregnancy, err := h.db.GetPregnancyByID(ctx, pregnancyID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pregnancy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Only owner can set outcome
	if pregnancy.OwnerID != user.UserID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only owner can set outcome")
		return
	}

	// Check if archived
	if pregnancy.Archived {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Cannot modify archived pregnancy")
		return
	}

	var req models.OutcomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Validate outcome
	validOutcomes := map[string]bool{"ongoing": true, "birth": true, "miscarriage": true, "ectopic": true, "stillbirth": true}
	if !validOutcomes[req.Outcome] {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid outcome value")
		return
	}

	updated, err := h.db.SetPregnancyOutcome(ctx, pregnancyID, req.Outcome, req.OutcomeDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(updated),
		Role:       "owner",
		Permission: "write",
	}
	writeJSON(w, http.StatusOK, resp)
}

// SetPregnancyArchive archives or unarchives a pregnancy.
func (h *Handler) SetPregnancyArchive(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	pregnancyID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pregnancy ID")
		return
	}

	pregnancy, err := h.db.GetPregnancyByID(ctx, pregnancyID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pregnancy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Only owner can archive
	if pregnancy.OwnerID != user.UserID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only owner can archive")
		return
	}

	var req models.ArchiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	updated, err := h.db.SetPregnancyArchive(ctx, pregnancyID, req.Archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.PregnancyResponse{
		Pregnancy:  toPregnancyDTO(updated),
		Role:       "owner",
		Permission: "write",
	}
	writeJSON(w, http.StatusOK, resp)
}

// Entry endpoints

// GetEntries gets entries for the pregnancy.
func (h *Handler) GetEntries(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, _, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	entryType := r.URL.Query().Get("type")
	sinceStr := r.URL.Query().Get("since")
	includeDeleted := r.URL.Query().Get("includeDeleted") == "true"

	var since *time.Time
	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			since = &t
		}
	}

	entries, err := h.db.GetEntries(ctx, pregnancy.ID, entryType, since, includeDeleted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.EntriesResponse{
		Entries:     entries,
		SyncVersion: time.Now().UnixMilli(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateEntry creates a new entry.
func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	var req models.EntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	entry, err := h.db.UpsertEntry(ctx, pregnancy.ID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

// BatchCreateEntries creates multiple entries.
func (h *Handler) BatchCreateEntries(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	var req models.BatchEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	var entries []models.Entry
	for _, e := range req.Entries {
		entry, err := h.db.UpsertEntry(ctx, pregnancy.ID, &e)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		entries = append(entries, *entry)
	}

	resp := models.EntriesResponse{
		Entries:     entries,
		SyncVersion: time.Now().UnixMilli(),
	}
	writeJSON(w, http.StatusCreated, resp)
}

// DeleteEntry soft deletes an entry.
func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	clientID := vars["clientId"]

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	err = h.db.DeleteEntry(ctx, pregnancy.ID, clientID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entry not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"deletedAt": time.Now().Format(time.RFC3339),
	})
}

// Settings endpoints

// GetSettings gets all settings.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, _, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	settings, err := h.db.GetSettings(ctx, pregnancy.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
}

// UpdateSetting updates a specific setting.
func (h *Handler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	settingType := vars["type"]

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read body")
		return
	}

	err = h.db.UpsertSetting(ctx, pregnancy.ID, settingType, json.RawMessage(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Sync endpoints

// GetSync returns all data since last sync.
func (h *Handler) GetSync(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, _, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		// No pregnancy yet - return empty sync
		writeJSON(w, http.StatusOK, models.SyncResponse{
			SyncVersion: time.Now().UnixMilli(),
			ServerTime:  time.Now().Format(time.RFC3339),
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	sinceStr := r.URL.Query().Get("since")
	var since *time.Time
	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			since = &t
		}
	}

	// Get all entries grouped by type
	entries, err := h.db.GetEntries(ctx, pregnancy.ID, "", since, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	entriesByType := make(map[string][]models.Entry)
	for _, e := range entries {
		entriesByType[e.EntryType] = append(entriesByType[e.EntryType], e)
	}

	settings, err := h.db.GetSettings(ctx, pregnancy.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.SyncResponse{
		Pregnancy:   toPregnancyDTO(pregnancy),
		Entries:     entriesByType,
		Settings:    settings,
		SyncVersion: time.Now().UnixMilli(),
		ServerTime:  time.Now().Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, resp)
}

// PostSync pushes local changes to server.
func (h *Handler) PostSync(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	var req models.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Get or create pregnancy
	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound && req.Pregnancy != nil {
		// Create new pregnancy
		pregnancy, err = h.db.CreatePregnancy(ctx, user.UserID, req.Pregnancy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		permission = "write"
	} else if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	// Update pregnancy if provided
	if req.Pregnancy != nil && pregnancy != nil {
		pregnancy, err = h.db.UpdatePregnancy(ctx, pregnancy.ID, req.Pregnancy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
	}

	// Upsert entries
	for _, e := range req.Entries {
		_, err := h.db.UpsertEntry(ctx, pregnancy.ID, &e)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
	}

	// Delete entries
	for _, clientID := range req.DeletedEntries {
		h.db.DeleteEntry(ctx, pregnancy.ID, clientID)
	}

	// Update settings
	for settingType, data := range req.Settings {
		err := h.db.UpsertSetting(ctx, pregnancy.ID, settingType, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
	}

	// Update sync state
	syncVersion := time.Now().UnixMilli()
	h.db.UpdateSyncState(ctx, user.UserID, req.DeviceID, syncVersion)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"conflicts":   []interface{}{},
		"syncVersion": syncVersion,
	})
}

// Pairing endpoints

// CreatePairingRequest creates a new pairing request.
func (h *Handler) CreatePairingRequest(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	var req models.PairingRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.TargetEmail == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Target email required")
		return
	}

	pr, err := h.db.CreatePairingRequest(ctx, user.UserID, req.RequesterName, req.TargetEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"requestId": pr.ID,
		"status":    pr.Status,
		"message":   "Request sent. Waiting for approval.",
	})
}

// GetPendingPairingRequests gets pending requests for the user.
func (h *Handler) GetPendingPairingRequests(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	requests, err := h.db.GetPendingPairingRequests(ctx, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"requests": requests})
}

// ApprovePairingRequest approves a pairing request.
func (h *Handler) ApprovePairingRequest(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	requestID, _ := strconv.ParseInt(vars["requestId"], 10, 64)

	var req models.ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Permission != "read" && req.Permission != "write" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Permission must be 'read' or 'write'")
		return
	}

	err := h.db.ApprovePairingRequest(ctx, requestID, user.UserID, req.Permission)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Request not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// DenyPairingRequest denies a pairing request.
func (h *Handler) DenyPairingRequest(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	requestID, _ := strconv.ParseInt(vars["requestId"], 10, 64)

	err := h.db.DenyPairingRequest(ctx, requestID, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Request not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// UpdatePartnerPermission updates partner's permission level.
func (h *Handler) UpdatePartnerPermission(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	var req models.PermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Permission != "read" && req.Permission != "write" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Permission must be 'read' or 'write'")
		return
	}

	err := h.db.UpdatePartnerPermission(ctx, user.UserID, req.Permission)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No partner paired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// RemovePairing removes a pairing.
func (h *Handler) RemovePairing(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	err := h.db.RemovePairing(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pairing found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetPairingStatus gets current pairing status.
func (h *Handler) GetPairingStatus(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	// Check as owner
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, user.UserID)
	if err == nil {
		resp := models.PairingStatusResponse{
			Paired: pregnancy.PartnerID.Valid,
			Role:   "owner",
		}
		if pregnancy.PartnerID.Valid {
			resp.Partner = &models.PartnerInfo{
				ID:         pregnancy.PartnerID.String,
				Permission: pregnancy.PartnerPermission.String,
				PairedAt:   pregnancy.UpdatedAt.Format(time.RFC3339),
			}
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Check as partner
	pregnancy, err = h.db.GetPregnancyByPartner(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeJSON(w, http.StatusOK, models.PairingStatusResponse{
			Paired: false,
			Role:   "",
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.PairingStatusResponse{
		Paired: true,
		Role:   "partner",
		Partner: &models.PartnerInfo{
			ID:         pregnancy.OwnerID,
			Permission: pregnancy.PartnerPermission.String,
			PairedAt:   pregnancy.UpdatedAt.Format(time.RFC3339),
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// ============ Invite Code / Sharing Endpoints ============

// GetSharingStatus gets the current sharing status for the owner.
func (h *Handler) GetSharingStatus(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	// Only owner can view sharing status
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Get partner info
	var partner *models.PartnerInfo
	if pregnancy.PartnerID.Valid {
		displayCard := true
		if pregnancy.DisplayPartnerCard.Valid {
			displayCard = pregnancy.DisplayPartnerCard.Bool
		}
		partner = &models.PartnerInfo{
			ID:                 pregnancy.PartnerID.String,
			Permission:         pregnancy.PartnerPermission.String,
			PairedAt:           pregnancy.UpdatedAt.Format(time.RFC3339),
			DisplayPartnerCard: displayCard,
		}
	}

	// Get supporters
	supporters, err := h.db.GetSupporters(ctx, pregnancy.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	supporterInfos := make([]models.SupporterInfo, 0, len(supporters))
	for _, s := range supporters {
		displayName := ""
		if s.DisplayName.Valid {
			displayName = s.DisplayName.String
		}
		displayCard := true
		if s.DisplayPartnerCard.Valid {
			displayCard = s.DisplayPartnerCard.Bool
		}
		supporterInfos = append(supporterInfos, models.SupporterInfo{
			ID:                 s.ID,
			UserID:             s.UserID,
			DisplayName:        displayName,
			JoinedAt:           s.JoinedAt.Format(time.RFC3339),
			DisplayPartnerCard: displayCard,
		})
	}

	// Get active codes
	codes, err := h.db.GetActiveInviteCodes(ctx, pregnancy.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	activeCodeInfos := make([]models.ActiveCodeInfo, 0, len(codes))
	for _, c := range codes {
		activeCodeInfos = append(activeCodeInfos, models.ActiveCodeInfo{
			ID:         c.ID,
			CodePrefix: c.CodePrefix,
			Role:       c.Role,
			ExpiresAt:  c.ExpiresAt.Format(time.RFC3339),
			ExpiresIn:  FormatExpiresIn(c.ExpiresAt),
		})
	}

	resp := models.SharingStatus{
		Partner:     partner,
		Supporters:  supporterInfos,
		ActiveCodes: activeCodeInfos,
	}
	writeJSON(w, http.StatusOK, resp)
}

// GenerateInviteCode generates a new invite code.
func (h *Handler) GenerateInviteCode(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	// Only owner can generate codes
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only pregnancy owner can generate codes")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var req models.GenerateCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Validate role
	if req.Role != "father" && req.Role != "support" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Role must be 'father' or 'support'")
		return
	}

	// Check if already has partner for father role
	if req.Role == "father" && pregnancy.PartnerID.Valid {
		writeError(w, http.StatusConflict, "CONFLICT", "Already has a partner")
		return
	}

	// Default permission to read
	permission := req.Permission
	if permission == "" {
		permission = "read"
	}
	if permission != "read" && permission != "write" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Permission must be 'read' or 'write'")
		return
	}

	// Generate code
	code, err := GenerateInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Hash code for storage
	codeHash, err := HashCode(code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Save code
	expiresAt := time.Now().Add(CodeExpiration)
	codeRecord, err := h.db.CreateInviteCode(ctx, pregnancy.ID, codeHash, GetCodePrefix(code), req.Role, permission, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.GenerateCodeResponse{
		Code:      code,
		ExpiresAt: codeRecord.ExpiresAt,
		Role:      req.Role,
	}
	writeJSON(w, http.StatusCreated, resp)
}

// RedeemInviteCode redeems an invite code.
func (h *Handler) RedeemInviteCode(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	var req models.RedeemCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Rate limit check (5 attempts per hour)
	attempts, err := h.db.CountRecentCodeAttempts(ctx, user.UserID)
	if err == nil && attempts >= 5 {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many attempts. Try again later.")
		return
	}

	// Validate code format
	if !IsValidCodeFormat(req.Code) {
		h.db.RecordCodeAttempt(ctx, user.UserID, false, r.RemoteAddr)
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid code format")
		return
	}

	// Find matching code by iterating through active codes
	activeCodes, err := h.db.FindActiveInviteCodes(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var matchedCode *models.InviteCode
	for _, c := range activeCodes {
		if VerifyCode(req.Code, c.CodeHash) {
			matchedCode = &c
			break
		}
	}

	if matchedCode == nil {
		h.db.RecordCodeAttempt(ctx, user.UserID, false, r.RemoteAddr)
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Invalid or expired code")
		return
	}

	// Redeem the code (email is used to check for admin access)
	pregnancy, actualPermission, err := h.db.RedeemInviteCode(ctx, matchedCode.ID, user.UserID, req.DisplayName, req.Email)
	if err == db.ErrNotFound {
		h.db.RecordCodeAttempt(ctx, user.UserID, false, r.RemoteAddr)
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Code already redeemed or expired")
		return
	}
	if err != nil {
		h.db.RecordCodeAttempt(ctx, user.UserID, false, r.RemoteAddr)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Record successful attempt
	h.db.RecordCodeAttempt(ctx, user.UserID, true, r.RemoteAddr)

	// Build response
	dueDate := ""
	if pregnancy.DueDate.Valid {
		dueDate = pregnancy.DueDate.Time.Format("2006-01-02")
	}
	momName := ""
	if pregnancy.MomName.Valid {
		momName = pregnancy.MomName.String
	}
	babyName := ""
	if pregnancy.BabyName.Valid {
		babyName = pregnancy.BabyName.String
	}

	resp := models.RedeemCodeResponse{
		Success:    true,
		Role:       matchedCode.Role,
		Permission: actualPermission,
		Pregnancy:  toPregnancyDTO(pregnancy),
		MomName:    momName,
		BabyName:   babyName,
		DueDate:    dueDate,
	}
	writeJSON(w, http.StatusOK, resp)
}

// RevokeInviteCode revokes an active invite code.
func (h *Handler) RevokeInviteCode(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	codeID, err := strconv.ParseInt(vars["codeId"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid code ID")
		return
	}

	err = h.db.RevokeInviteCode(ctx, codeID, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Code not found or already revoked")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// RemoveSupporter removes a supporter.
func (h *Handler) RemoveSupporter(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	supporterID, err := strconv.ParseInt(vars["supporterId"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid supporter ID")
		return
	}

	err = h.db.RemoveSupporter(ctx, supporterID, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Supporter not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetMyRole returns the user's role and permission for any accessible pregnancy.
func (h *Handler) GetMyRole(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	// Try as owner first
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, user.UserID)
	if err == nil {
		resp := models.MyRoleResponse{
			Role:       "owner",
			Permission: "write",
			Pregnancy:  toPregnancyDTO(pregnancy),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err != nil && err != db.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Try as coowner (admin with owner-level access)
	pregnancy, err = h.db.GetPregnancyByCoowner(ctx, user.UserID)
	if err == nil {
		resp := models.MyRoleResponse{
			Role:       "coowner",
			Permission: "write",
			Pregnancy:  toPregnancyDTO(pregnancy),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err != nil && err != db.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Try as partner
	pregnancy, err = h.db.GetPregnancyByPartner(ctx, user.UserID)
	if err == nil {
		permission := "read"
		if pregnancy.PartnerPermission.Valid {
			permission = pregnancy.PartnerPermission.String
		}
		resp := models.MyRoleResponse{
			Role:       "father",
			Permission: permission,
			Pregnancy:  toPregnancyDTO(pregnancy),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err != nil && err != db.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Try as supporter
	pregnancy, err = h.db.GetPregnancyBySupporter(ctx, user.UserID)
	if err == nil {
		// Get supporter record to check permission
		supporter, sErr := h.db.GetSupporterByUserID(ctx, user.UserID)
		permission := "read"
		if sErr == nil && supporter.Permission.Valid {
			permission = supporter.Permission.String
		}
		resp := models.MyRoleResponse{
			Role:       "support",
			Permission: permission,
			Pregnancy:  toPregnancyDTO(pregnancy),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err != nil && err != db.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// No access
	resp := models.MyRoleResponse{
		Role:       "",
		Permission: "",
		Pregnancy:  nil,
	}
	writeJSON(w, http.StatusOK, resp)
}

// File endpoints

// UploadFile handles file uploads.
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()

	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No pregnancy found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	// Parse multipart form (max 10MB)
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No file uploaded")
		return
	}
	defer file.Close()

	fileType := r.FormValue("fileType")
	clientID := r.FormValue("clientId")
	metadataStr := r.FormValue("metadata")

	// Create storage path
	now := time.Now()
	storagePath := filepath.Join(
		fmt.Sprintf("%d", pregnancy.ID),
		fileType,
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%d_%s", now.UnixNano(), header.Filename),
	)

	fullPath := filepath.Join(h.uploadPath, storagePath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create directory")
		return
	}

	// Save file
	dst, err := os.Create(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create file")
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save file")
		return
	}

	// Create file record
	f := &models.File{
		FileType:    fileType,
		StoragePath: storagePath,
		SizeBytes:   sql.NullInt64{Int64: size, Valid: true},
	}
	if clientID != "" {
		f.ClientID = sql.NullString{String: clientID, Valid: true}
	}
	if metadataStr != "" {
		f.Metadata = json.RawMessage(metadataStr)
	}

	// Detect mime type from header
	contentType := header.Header.Get("Content-Type")
	if contentType != "" {
		f.MimeType = sql.NullString{String: contentType, Valid: true}
	}

	fileRecord, err := h.db.CreateFile(ctx, pregnancy.ID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"fileId": fileRecord.ID,
		"url":    fmt.Sprintf("/files/%s", storagePath),
	})
}

// GetFile gets file metadata.
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID, _ := strconv.ParseInt(vars["fileId"], 10, 64)

	file, err := h.db.GetFile(ctx, fileID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Verify access
	pregnancy, err := h.db.GetPregnancyByID(ctx, file.PregnancyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if pregnancy.OwnerID != user.UserID &&
		(!pregnancy.PartnerID.Valid || pregnancy.PartnerID.String != user.UserID) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
		return
	}

	writeJSON(w, http.StatusOK, file)
}

// DeleteFile deletes a file.
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	user := getUserInfo(r)
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID, _ := strconv.ParseInt(vars["fileId"], 10, 64)

	file, err := h.db.GetFile(ctx, fileID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Verify access
	pregnancy, permission, err := h.getAccessiblePregnancy(ctx, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if pregnancy.ID != file.PregnancyID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
		return
	}

	if permission != "write" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No write permission")
		return
	}

	err = h.db.DeleteFile(ctx, fileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Helper functions

func (h *Handler) getAccessiblePregnancy(ctx context.Context, userID string) (*models.Pregnancy, string, error) {
	// Try as owner first
	pregnancy, err := h.db.GetPregnancyByOwner(ctx, userID)
	if err == nil {
		return pregnancy, "write", nil
	}
	if err != db.ErrNotFound {
		return nil, "", err
	}

	// Try as coowner (admin with owner-level access)
	pregnancy, err = h.db.GetPregnancyByCoowner(ctx, userID)
	if err == nil {
		return pregnancy, "write", nil
	}
	if err != db.ErrNotFound {
		return nil, "", err
	}

	// Try as partner
	pregnancy, err = h.db.GetPregnancyByPartner(ctx, userID)
	if err == nil {
		permission := "read"
		if pregnancy.PartnerPermission.Valid {
			permission = pregnancy.PartnerPermission.String
		}
		return pregnancy, permission, nil
	}
	if err != db.ErrNotFound {
		return nil, "", err
	}

	// Try as supporter
	pregnancy, err = h.db.GetPregnancyBySupporter(ctx, userID)
	if err == nil {
		// Get supporter record to check permission
		supporter, sErr := h.db.GetSupporterByUserID(ctx, userID)
		permission := "read"
		if sErr == nil && supporter.Permission.Valid {
			permission = supporter.Permission.String
		}
		return pregnancy, permission, nil
	}

	return nil, "", err
}

func toPregnancyDTO(p *models.Pregnancy) *models.PregnancyDTO {
	dto := &models.PregnancyDTO{
		ID:          p.ID,
		OwnerID:     p.OwnerID,
		CycleLength: p.CycleLength,
		Archived:    p.Archived,
	}

	if p.PartnerID.Valid {
		dto.PartnerID = &p.PartnerID.String
	}
	if p.PartnerPermission.Valid {
		dto.PartnerPermission = &p.PartnerPermission.String
	}
	if p.DueDate.Valid {
		s := p.DueDate.Time.Format("2006-01-02")
		dto.DueDate = &s
	}
	if p.StartDate.Valid {
		s := p.StartDate.Time.Format("2006-01-02")
		dto.StartDate = &s
	}
	if p.CalculationMethod.Valid {
		dto.CalculationMethod = &p.CalculationMethod.String
	}
	if p.BabyName.Valid {
		dto.BabyName = &p.BabyName.String
	}
	if p.MomName.Valid {
		dto.MomName = &p.MomName.String
	}
	if p.MomBirthday.Valid {
		s := p.MomBirthday.Time.Format("2006-01-02")
		dto.MomBirthday = &s
	}
	if p.Gender.Valid {
		dto.Gender = &p.Gender.String
	}
	if p.ParentRole.Valid {
		dto.ParentRole = &p.ParentRole.String
	}
	if p.ProfilePhoto.Valid {
		dto.ProfilePhoto = &p.ProfilePhoto.String
	}
	if p.Outcome.Valid {
		dto.Outcome = &p.Outcome.String
	}
	if p.OutcomeDate.Valid {
		s := p.OutcomeDate.Time.Format("2006-01-02")
		dto.OutcomeDate = &s
	}
	if p.ArchivedAt.Valid {
		s := p.ArchivedAt.Time.Format(time.RFC3339)
		dto.ArchivedAt = &s
	}

	return dto
}

// ============ Static Data Endpoints ============

// GetBabySizes returns the baby sizes JSON data.
func (h *Handler) GetBabySizes(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join(h.dataPath, "BabySizes.json")
	http.ServeFile(w, r, filePath)
}

// GetWeeklyFacts returns the weekly facts JSON data.
func (h *Handler) GetWeeklyFacts(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join(h.dataPath, "WeeklyFacts.json")
	http.ServeFile(w, r, filePath)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	resp := models.ErrorResponse{
		Error: models.ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	writeJSON(w, status, resp)
}
