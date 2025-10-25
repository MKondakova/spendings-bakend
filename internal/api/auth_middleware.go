package api

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"spendings-backend/internal/models"
)

var (
	errNicknameIsEmpty      = errors.New("nickname is empty")
	errUnauthorized         = errors.New("unauthorized")
	errForbidden            = errors.New("forbidden")
	errInvalidSigningMethod = errors.New("invalid signing method")
)

type AuthMiddleware struct {
	publicKey *rsa.PublicKey

	logger        *zap.SugaredLogger
	revokedTokens map[string]struct{}
}

func NewAuthMiddleware(
	publicKey *rsa.PublicKey,
	logger *zap.SugaredLogger,
	revokedTokensList []string,
) *AuthMiddleware {
	revokedTokens := make(map[string]struct{}, len(revokedTokensList))
	for _, token := range revokedTokensList {
		revokedTokens[token] = struct{}{}
	}

	return &AuthMiddleware{
		publicKey:     publicKey,
		logger:        logger,
		revokedTokens: revokedTokens,
	}
}

func (m *AuthMiddleware) JWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		claims, err := m.Check(request.Header.Get("Authorization"), request.URL.Path)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")

			m.logger.Errorf("can't check JWT: %s, payload: %s", err, m.payload(request))

			var errRes error
			if errors.Is(err, errForbidden) {
				response.WriteHeader(http.StatusForbidden)
				_, errRes = response.Write([]byte(`{"error": "forbidden"}`))
			} else {
				response.WriteHeader(http.StatusUnauthorized)
				_, errRes = response.Write([]byte(`{"error": "unauthorized"}`))
			}

			if errRes != nil {
				m.logger.Errorf("can't write response: %s, payload: %s", errRes, m.payload(request))
			}

			return
		}

		next.ServeHTTP(response, request.WithContext(ContextWithClaims(request.Context(), claims)))
	}
}

func (m *AuthMiddleware) payload(request *http.Request) string {
	aHdr := request.Header.Get("Authorization")
	aHdrParts := strings.Split(aHdr, ".")

	if len(aHdrParts) > 1 {
		payloadDecoded, err := base64.RawStdEncoding.DecodeString(aHdrParts[1])
		if err != nil {
			return ""
		}

		return string(payloadDecoded)
	}

	return ""
}

func ContextWithClaims(ctx context.Context, claims *models.AuthTokenClaims) context.Context {
	return context.WithValue(ctx, models.ContextClaimsKey{}, claims)
}

func (m *AuthMiddleware) Check(serviceJWT, requestedMethod string) (*models.AuthTokenClaims, error) {
	jwtAuthPrefix := "Bearer "

	if !strings.HasPrefix(serviceJWT, jwtAuthPrefix) {
		return nil, fmt.Errorf("auth header is invalid: %w", errUnauthorized)
	}

	tokenString := serviceJWT[len(jwtAuthPrefix):]

	claims, err := m.parse(tokenString)
	if err != nil {
		return nil, fmt.Errorf("can't parse JWT: %w", err)
	}

	if m.isRevoked(claims.ID) {
		return nil, fmt.Errorf(
			"%w: revoked token with nickname %s and id %s",
			errForbidden,
			claims.Nickname,
			claims.ID,
		)
	}

	if requestedMethod == "/api/generate-token" {
		if !claims.IsTeacher {
			return nil, fmt.Errorf(
				"%w: access denied for token with nickname %s and id %s",
				errForbidden,
				claims.Nickname,
				claims.ID,
			)
		}
	}

	return claims, nil
}

func (m *AuthMiddleware) isRevoked(id string) bool {
	_, has := m.revokedTokens[id]

	return has
}

func (m *AuthMiddleware) parse(token string) (*models.AuthTokenClaims, error) {
	parser := jwt.NewParser()

	var claims models.AuthTokenClaims

	_, err := parser.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errInvalidSigningMethod
		}

		return m.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't parse token: %w", err)
	}

	err = m.validate(&claims)
	if err != nil {
		return nil, fmt.Errorf("claims is invalid: %w", err)
	}

	return &claims, nil
}

func (m *AuthMiddleware) validate(claims *models.AuthTokenClaims) error {
	if claims.Nickname == "" {
		return errNicknameIsEmpty
	}

	return nil
}
