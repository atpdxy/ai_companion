package service

import (
	"testing"
	"time"

	"companion/api/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidation(t *testing.T) {
	a := &Auth{Secret: []byte("01234567890123456789012345678901")}
	token, err := a.issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	user, err := a.Verify(token)
	if err != nil || user != "user-1" {
		t.Fatalf("valid token rejected: %v", err)
	}
	if _, err = a.Verify(token + "x"); err != domain.ErrUnauthorized {
		t.Fatal("tampered token accepted")
	}
	claims := jwt.RegisteredClaims{Subject: "user-1", Issuer: "companion-api", Audience: jwt.ClaimStrings{"companion-web"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Verify(expired); err != domain.ErrUnauthorized {
		t.Fatal("expired token accepted")
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	claims.Audience = jwt.ClaimStrings{"other-app"}
	wrong, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Verify(wrong); err != domain.ErrUnauthorized {
		t.Fatal("wrong audience accepted")
	}
}
