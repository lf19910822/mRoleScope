package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionLifetime    = 3 * 24 * time.Hour
	accessTokenVersion = "v1"
)

var (
	ErrInvalidInvite  = errors.New("invalid invite")
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("expired session")
	secretPattern     = regexp.MustCompile(`^[0-9a-fA-F]{64,}$`)
)

type Config struct {
	InviteBcryptHash     string
	SessionSigningSecret string
	Now                  func() time.Time
}

type Service struct {
	inviteHash []byte
	secret     []byte
	now        func() time.Time
}

func New(config Config) (*Service, error) {
	if err := validateBcryptHash(config.InviteBcryptHash); err != nil {
		return nil, err
	}
	if !secretPattern.MatchString(config.SessionSigningSecret) {
		return nil, fmt.Errorf("session signing secret must be at least 32 bytes encoded as hexadecimal")
	}
	secret, err := hex.DecodeString(config.SessionSigningSecret)
	if err != nil || len(secret) < 32 {
		return nil, fmt.Errorf("session signing secret must be at least 32 bytes encoded as hexadecimal")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Service{inviteHash: []byte(config.InviteBcryptHash), secret: secret, now: now}, nil
}

func validateBcryptHash(hash string) error {
	if len(hash) < 59 {
		return fmt.Errorf("invite bcrypt hash is invalid")
	}
	if _, err := bcrypt.Cost([]byte(hash)); err != nil {
		return fmt.Errorf("invite bcrypt hash is invalid")
	}
	return nil
}

func (service *Service) Issue(invite string) (string, error) {
	if err := bcrypt.CompareHashAndPassword(service.inviteHash, []byte(invite)); err != nil {
		return "", ErrInvalidInvite
	}
	expiresAt := service.now().Add(SessionLifetime).Unix()
	payload := strings.Join([]string{accessTokenVersion, strconv.FormatInt(expiresAt, 10), service.hashFingerprint()}, ".")
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := service.sign(encodedPayload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (service *Service) Validate(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ErrInvalidSession
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || subtle.ConstantTimeCompare(signature, service.sign(parts[0])) != 1 {
		return ErrInvalidSession
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrInvalidSession
	}
	payload := strings.Split(string(payloadBytes), ".")
	if len(payload) != 3 || payload[0] != accessTokenVersion || payload[2] != service.hashFingerprint() {
		return ErrInvalidSession
	}
	expiresAt, err := strconv.ParseInt(payload[1], 10, 64)
	if err != nil {
		return ErrInvalidSession
	}
	if expiresAt <= service.now().Unix() {
		return ErrExpiredSession
	}
	return nil
}

func (service *Service) sign(payload string) []byte {
	hash := hmac.New(sha256.New, service.secret)
	_, _ = hash.Write([]byte(payload))
	return hash.Sum(nil)
}

func (service *Service) hashFingerprint() string {
	hash := sha256.Sum256(service.inviteHash)
	return hex.EncodeToString(hash[:])
}
