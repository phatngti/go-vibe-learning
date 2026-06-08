package csrf

import (
	"crypto/sha256"
	"encoding/base64"
	"io"

	"github/master_go_cqrs/internal/observability/logger"

	"go.uber.org/zap"
)

const (
	Header = "X-CSRF-Token"
	// 32 bytes
	csrfSalt = "KbWaoi5xtDC3GEfBa9ovQdzOzXsuVU9I"
)

// Create CSRF token
func MakeToken(sid string, logger logger.Factory) string {
	hash := sha256.New()
	_, err := io.WriteString(hash, csrfSalt+sid)
	if err != nil {
		logger.Bg().Error("Make CSRF Token", zap.Error(err))
	}
	token := base64.RawStdEncoding.EncodeToString(hash.Sum(nil))
	return token
}

// Validate CSRF token
func ValidateToken(token string, sid string, logger logger.Factory) bool {
	trueToken := MakeToken(sid, logger)
	return token == trueToken
}
