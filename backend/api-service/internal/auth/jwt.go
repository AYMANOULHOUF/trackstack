package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// Claims carried in both access and refresh JWTs. TokenType distinguishes
// the two so a refresh token can't be used directly as an access token.
// IsAdmin marks the single global admin (no org); OrgID is empty for the admin.
type Claims struct {
	UserID    string `json:"uid"`
	OrgID     string `json:"org_id"`
	IsAdmin   bool   `json:"adm"`
	TokenType string `json:"typ"` // "access" | "refresh"
	jwt.RegisteredClaims
}

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
)

type JWTIssuer struct {
	secret []byte
}

func NewJWTIssuer(secret string) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret)}
}

func (j *JWTIssuer) issue(userID, orgID string, isAdmin bool, tokenType string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		OrgID:     orgID,
		IsAdmin:   isAdmin,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTIssuer) IssueAccessToken(userID, orgID string, isAdmin bool) (string, error) {
	return j.issue(userID, orgID, isAdmin, "access", accessTTL)
}

func (j *JWTIssuer) IssueRefreshToken(userID, orgID string, isAdmin bool) (string, error) {
	return j.issue(userID, orgID, isAdmin, "refresh", refreshTTL)
}

func (j *JWTIssuer) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
