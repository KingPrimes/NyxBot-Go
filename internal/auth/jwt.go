// Package auth 双令牌 JWT 实现（access 2h / refresh 7d），支持进程内黑名单吊销。
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrMissingToken      = errors.New("missing token")      // 缺少令牌
	ErrInvalidToken      = errors.New("invalid token")      // 无效令牌
	ErrExpiredToken      = errors.New("expired token")      // 令牌过期
	ErrUnsupportedToken  = errors.New("unsupported token")  // 不支持的令牌类型
	ErrBlacklistedToken  = errors.New("blacklisted token")  // 令牌已被吊销
)

// Claims JWT 载荷，包含用户 ID、用户名和令牌类型。
type Claims struct {
	UserID    uint   `json:"userId"`
	UserName  string `json:"userName"`
	TokenType string `json:"typ"` // access / refresh
	jwt.RegisteredClaims
}

// TokenPair access token 和 refresh token 的配对。
type TokenPair struct {
	Token        string `json:"token"`        // access token（2h 有效）
	RefreshToken string `json:"refreshToken"` // refresh token（7d 有效）
}

// Manager JWT 管理器，负责生成、解析和吊销令牌。
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration

	blacklistMu sync.Mutex
	blacklist   map[string]time.Time
}

// NewManager 创建 JWT 管理器，secret 为空时使用默认密钥。
func NewManager(secret string) *Manager {
	if strings.TrimSpace(secret) == "" {
		secret = "nyxbot-secret-key"
	}

	return &Manager{
		secret:     []byte(secret),
		accessTTL:  2 * time.Hour,
		refreshTTL: 7 * 24 * time.Hour,
		blacklist:  make(map[string]time.Time),
	}
}

// GeneratePair 生成 access + refresh 令牌对。
func (m *Manager) GeneratePair(userID uint, userName string) (TokenPair, error) {
	access, err := m.generate(userID, userName, TokenTypeAccess, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}

	refresh, err := m.generate(userID, userName, TokenTypeRefresh, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{Token: access, RefreshToken: refresh}, nil
}

// GenerateAccessToken 仅生成 access token。
func (m *Manager) GenerateAccessToken(userID uint, userName string) (string, error) {
	return m.generate(userID, userName, TokenTypeAccess, m.accessTTL)
}

// ParseAccessToken 解析并验证 access token（含过期检查）。
func (m *Manager) ParseAccessToken(raw string) (*Claims, error) {
	claims, err := m.parse(raw, false)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrUnsupportedToken
	}

	return claims, nil
}

// ParseAccessTokenAllowExpired 解析 access token，允许令牌已过期（用于 refresh 场景）。
func (m *Manager) ParseAccessTokenAllowExpired(raw string) (*Claims, error) {
	claims, err := m.parse(raw, true)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrUnsupportedToken
	}

	return claims, nil
}

// Blacklist 将令牌加入黑名单，到期后自动清理。
func (m *Manager) Blacklist(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}

	expiresAt := time.Now().Add(m.accessTTL)
	if claims, err := m.parse(raw, true); err == nil && claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	m.blacklistMu.Lock()
	defer m.blacklistMu.Unlock()
	m.blacklist[raw] = expiresAt
	m.cleanupBlacklistLocked(time.Now())
}

// IsBlacklisted 检查令牌是否在黑名单中。
func (m *Manager) IsBlacklisted(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	now := time.Now()
	m.blacklistMu.Lock()
	defer m.blacklistMu.Unlock()
	m.cleanupBlacklistLocked(now)

	_, ok := m.blacklist[raw]
	return ok
}

func (m *Manager) generate(userID uint, userName, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	jti, err := randomJTI()
	if err != nil {
		return "", err
	}

	claims := Claims{
		UserID:    userID,
		UserName:  userName,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    "nyxbot-go",
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) parse(raw string, allowExpired bool) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrMissingToken
	}
	if m.IsBlacklisted(raw) {
		return nil, ErrBlacklistedToken
	}

	claims := &Claims{}
	options := []jwt.ParserOption{jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})}
	if allowExpired {
		options = append(options, jwt.WithoutClaimsValidation())
	}

	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, options...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *Manager) cleanupBlacklistLocked(now time.Time) {
	for token, expiresAt := range m.blacklist {
		if !expiresAt.After(now) {
			delete(m.blacklist, token)
		}
	}
}

func randomJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// BearerToken 从 Authorization 请求头中提取 Bearer token。
func BearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}

	return header
}
