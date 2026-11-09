package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"companion/api/internal/domain"
	"companion/api/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	Store  repository.Store
	Secret []byte
}

func (a *Auth) Register(ctx context.Context, email, password string, adult bool) (domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || len(password) < 10 || len(password) > 72 || !adult {
		return domain.User{}, "", domain.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return domain.User{}, "", err
	}
	u, err := a.Store.CreateUser(ctx, email, string(hash))
	if err != nil {
		return u, "", err
	}
	token, err := a.issue(u.ID)
	return u, token, err
}
func (a *Auth) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	if len(password) > 72 {
		return domain.User{}, "", domain.ErrUnauthorized
	}
	u, err := a.Store.FindUser(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, domain.ErrNotFound) {
		// Comparable work on unknown accounts reduces trivial timing enumeration.
		if _, hashErr := bcrypt.GenerateFromPassword([]byte("invalid-account-password"), 12); hashErr != nil {
			return u, "", hashErr
		}
		return u, "", domain.ErrUnauthorized
	}
	if err != nil {
		return u, "", err
	}
	if err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return u, "", domain.ErrUnauthorized
	}
	token, err := a.issue(u.ID)
	return u, token, err
}
func (a *Auth) issue(user string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{Subject: user, Issuer: "companion-api", Audience: jwt.ClaimStrings{"companion-web"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour))}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.Secret)
}
func (a *Auth) Verify(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (any, error) { return a.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("companion-api"), jwt.WithAudience("companion-web"), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid || claims.Subject == "" {
		return "", domain.ErrUnauthorized
	}
	return claims.Subject, nil
}
