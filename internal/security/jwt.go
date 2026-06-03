package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type AccessTokenClaims struct {
	Subject string `json:"sub"`
	Expires int64  `json:"exp"`
	Issued  int64  `json:"iat"`
	Type    string `json:"typ"`
}

func SignAccessToken(subject string, ttl time.Duration, secretKey string) (string, error) {
	now := time.Now().UTC()
	claims := AccessTokenClaims{
		Subject: subject,
		Expires: now.Add(ttl).Unix(),
		Issued:  now.Unix(),
		Type:    "access",
	}
	return signJWT(claims, secretKey)
}

func ValidateAccessToken(token, secretKey string) (AccessTokenClaims, error) {
	var claims AccessTokenClaims
	if err := parseJWT(token, secretKey, &claims); err != nil {
		return AccessTokenClaims{}, err
	}
	if claims.Type != "access" {
		return AccessTokenClaims{}, errors.New("invalid token type")
	}
	if time.Now().UTC().Unix() > claims.Expires {
		return AccessTokenClaims{}, errors.New("token expired")
	}
	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	return RandomToken(32)
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func signJWT(payload any, secretKey string) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := signJWTString(h+"."+p, secretKey)
	return h + "." + p + "." + sig, nil
}

func parseJWT(token, secretKey string, payload any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid token")
	}
	expected := signJWTString(parts[0]+"."+parts[1], secretKey)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return errors.New("invalid signature")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(decoded, payload); err != nil {
		return err
	}
	return nil
}

func signJWTString(signingInput, secretKey string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func parseNumericSubject(subject string) (int64, error) {
	if subject == "" {
		return 0, errors.New("missing subject")
	}
	return strconv.ParseInt(subject, 10, 64)
}
