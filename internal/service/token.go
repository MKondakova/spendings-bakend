package service

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"spendings-backend/internal/models"
)

type TokenService struct {
	privateKey       *rsa.PrivateKey
	keysListFilePath string
}

func NewTokenService(privateKey *rsa.PrivateKey, filepath string) *TokenService {
	return &TokenService{
		privateKey:       privateKey,
		keysListFilePath: filepath,
	}
}

func (t *TokenService) GenerateToken(ctx context.Context, username string, isTeacher bool) (string, error) {
	teacherData := models.ClaimsFromContext(ctx)

	if teacherData == nil {
		return "", fmt.Errorf("%w: teacherData is empty", models.ErrUnauthorized)
	}

	if !teacherData.IsTeacher {
		return "", fmt.Errorf("%w: teacherData is not teacher", models.ErrForbidden)
	}

	issuer := teacherData.Nickname

	claims := models.AuthTokenClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer: issuer,
			ID:     uuid.NewString(),
		},
		Nickname:  username,
		IsTeacher: isTeacher,
	}

	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	tokenString, err := token.SignedString(t.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	creationLog := fmt.Sprintf("%s;%s;%s;%t\n", issuer, username, claims.ID, isTeacher)
	err = AppendFile(t.keysListFilePath, []byte(creationLog), 0600)

	return tokenString, nil
}

func AppendFile(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}
