package dbrepo

import (
	"context"
	"time"
	"toomanyhours-api/internal/models"
)

func (m *PostgresDBRepo) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	return m.GormDB.WithContext(ctx).Create(token).Error
}

// GetRefreshToken returns gorm.ErrRecordNotFound when the jti is unknown, which
// the caller turns into the same 401 as every other failure.
func (m *PostgresDBRepo) GetRefreshToken(ctx context.Context, jti string) (*models.RefreshToken, error) {
	var token models.RefreshToken

	if err := m.GormDB.WithContext(ctx).Where("jti = ?", jti).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// RevokeRefreshToken consumes one token.
//
// The `revoked_at IS NULL` guard is load-bearing rather than defensive: on the
// grace path the row is already revoked, and moving revoked_at forward would
// let a replay every few seconds extend the window indefinitely — turning the
// thing that bounds the risk into the thing that removes it.
func (m *PostgresDBRepo) RevokeRefreshToken(ctx context.Context, jti string) error {
	return m.GormDB.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("jti = ? AND revoked_at IS NULL", jti).
		Update("revoked_at", time.Now()).Error
}

// FamilyHasActiveToken reports whether any token in the family is still alive.
//
// This is what separates a rotated token from an ended session. Rotation always
// leaves a live successor, so a replay of the consumed token inside the grace
// window is a second browser tab. Logout and reuse detection revoke everything,
// so nothing is alive and the grace window must not resurrect it.
func (m *PostgresDBRepo) FamilyHasActiveToken(ctx context.Context, familyID string) (bool, error) {
	var live int64

	err := m.GormDB.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL AND expires_at > ?", familyID, time.Now()).
		Count(&live).Error

	if err != nil {
		return false, err
	}
	return live > 0, nil
}

// RevokeRefreshFamily ends a whole session chain. Called on logout, and on
// reuse detection where two parties holding one token means it leaked.
func (m *PostgresDBRepo) RevokeRefreshFamily(ctx context.Context, familyID string) error {
	return m.GormDB.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", time.Now()).Error
}

// DeleteExpiredRefreshTokens is housekeeping, called on login. Revoked but
// unexpired rows are deliberately kept: deleting them would make a replay look
// like "never issued" rather than reuse, and lose the detection.
func (m *PostgresDBRepo) DeleteExpiredRefreshTokens(ctx context.Context) error {
	return m.GormDB.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.RefreshToken{}).Error
}
