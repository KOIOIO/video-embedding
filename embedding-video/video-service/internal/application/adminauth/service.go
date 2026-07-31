package adminauth

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid authentication token")
)

type Service struct {
	repo   Repository
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewService(repo Repository, secret string, ttl time.Duration) *Service {
	return &Service{repo: repo, secret: []byte(secret), ttl: ttl, now: time.Now}
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}
	admin, found, err := s.repo.FindActiveAdminByUsername(ctx, username)
	if err != nil {
		return LoginResult{}, err
	}
	if !found || !verifyPassword(admin.PasswordHash, password) {
		return LoginResult{}, ErrInvalidCredentials
	}
	token, expiresAt, err := s.issueToken(admin.ID)
	if err != nil {
		return LoginResult{}, err
	}
	admin.PasswordHash = ""
	return LoginResult{Token: token, ExpiresAt: expiresAt, Admin: admin}, nil
}

func verifyPassword(passwordHash, password string) bool {
	passwordHash = strings.TrimSpace(passwordHash)
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil {
		return true
	}
	legacyHash, err := hex.DecodeString(passwordHash)
	if err != nil || len(legacyHash) != md5.Size {
		return false
	}
	passwordMD5 := md5.Sum([]byte(password))
	return subtle.ConstantTimeCompare(legacyHash, passwordMD5[:]) == 1
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (Admin, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	}, jwt.WithTimeFunc(s.now), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return Admin{}, ErrInvalidToken
	}
	id, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || id == 0 {
		return Admin{}, ErrInvalidToken
	}
	admin, found, err := s.repo.FindActiveAdminByID(ctx, id)
	if err != nil {
		return Admin{}, err
	}
	if !found {
		return Admin{}, ErrInvalidToken
	}
	admin.PasswordHash = ""
	return admin, nil
}

func (s *Service) issueToken(adminID uint64) (string, time.Time, error) {
	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(adminID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return token, expiresAt, err
}
