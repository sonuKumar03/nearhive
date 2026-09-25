package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPasswordHashing(t *testing.T) {
	password := "Secret123!"
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	assert.True(t, CheckPasswordHash(password, hash))
	assert.False(t, CheckPasswordHash("WrongPassword", hash))
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	mgr := NewManager("test-jwt-secret-key-1234567890123456", 1*time.Hour)
	userID := uuid.New()

	tokenStr, err := mgr.GenerateToken(userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := mgr.ValidateToken(tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}

func TestJWTTokenValidation_InvalidSecret(t *testing.T) {
	mgr1 := NewManager("secret-key-one-123456789012345678", 1*time.Hour)
	mgr2 := NewManager("secret-key-two-123456789012345678", 1*time.Hour)

	tokenStr, _ := mgr1.GenerateToken(uuid.New())
	_, err := mgr2.ValidateToken(tokenStr)
	assert.Error(t, err)
}
