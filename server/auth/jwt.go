package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// Claims represents the payload of a JWT access token.
type Claims struct {
	UserID   string `json:"sub"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 30 * 24 * time.Hour
	RefreshTokenBytes    = 32
)

// GenerateSecret creates a random 256-bit HMAC signing key.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateAccessToken creates a JWT signed with HMAC-SHA256.
func CreateAccessToken(secret string, userID, username string, isAdmin bool) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		IsAdmin:   isAdmin,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(AccessTokenDuration).Unix(),
	}

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64URLEncode(payload)

	signingInput := header + "." + payloadB64
	sig := signHMAC(secret, signingInput)

	return signingInput + "." + sig, nil
}

// ValidateAccessToken verifies a JWT and returns the claims.
func ValidateAccessToken(secret, token string) (*Claims, error) {
	parts := splitToken(token)
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := signHMAC(secret, signingInput)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, ErrTokenInvalid
	}

	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashRefreshToken returns the SHA256 hash of a refresh token for storage.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func signHMAC(secret, input string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return base64URLEncode(mac.Sum(nil))
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func splitToken(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}

// CreateTokenPair generates both an access token and a refresh token.
func CreateTokenPair(secret string, userID, username string, isAdmin bool) (*TokenPair, error) {
	accessToken, err := CreateAccessToken(secret, userID, username, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(AccessTokenDuration.Seconds()),
	}, nil
}
