package auth

import (
	"strings"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
)

const (
	AppTokenValidDuration = time.Hour * 24 * 7
)

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func RenderClaims(user *ent.User, duration time.Duration) jwtauth.RBACClaims[int64] {
	return jwtauth.NewRBACClaims(
		user.ID,
		user.Username,
		[]string{},
		time.Now().Add(duration),
	)
}
