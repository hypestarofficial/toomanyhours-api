package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

type Auth struct {
	Issuer             string
	Audience           string
	Secret             string
	TokenExpiry        time.Duration
	RefreshTokenExpiry time.Duration
	CookieDomain       string
	CookiePath         string
	CookieName         string
}

type jwtUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type TokenPairs struct {
	Token        string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateTokenPair signs an access token and a refresh token.
//
// refreshJTI must be the id of a row already written to refresh_tokens: the
// refresh token carries nothing else, so a token signed without its row would
// be permanently unusable.
//
// The access token is deliberately issued without an ID. /refresh-token
// requires one, so that asymmetry is what stops an access token — which
// travels in a header and is far more exposed — being presented as the
// refresh cookie.
func (j *Auth) GenerateTokenPair(user *jwtUser, refreshJTI string) (TokenPairs, error) {
	accessTokenClaims := Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			Audience:  []string{j.Audience},
			Issuer:    j.Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(j.TokenExpiry)),
		},
	}

	// Create a token with those claims
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)

	// Create a signed access token
	signedAccessToken, err := accessToken.SignedString([]byte(j.Secret))
	if err != nil {
		return TokenPairs{}, err
	}

	// Create claims for the refresh token (using standard claims)
	refreshTokenClaims := jwt.RegisteredClaims{
		ID: refreshJTI,
		// Kept for debugging only. The server reads the user from the row, not
		// from here — one source of truth, and the row is the revocable one.
		Subject:   fmt.Sprintf("%d", user.ID),
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(j.RefreshTokenExpiry)),
	}

	// Create a refresh token with those claims
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)

	// Create signed refresh token
	signedRefreshToken, err := refreshToken.SignedString([]byte(j.Secret))
	if err != nil {
		return TokenPairs{}, err
	}

	// Create TokenPairs and populate with signed tokens
	var tokenPairs = TokenPairs{
		Token:        signedAccessToken,
		RefreshToken: signedRefreshToken,
	}

	// Return TokenPairs
	return tokenPairs, nil
}

func (j *Auth) GetRefreshCookie(refreshToken string) *http.Cookie {
	return &http.Cookie{
		Name:     j.CookieName,
		Path:     "/",
		Value:    refreshToken,
		Expires:  time.Now().Add(j.RefreshTokenExpiry),
		MaxAge:   int(j.RefreshTokenExpiry.Seconds()),
		SameSite: http.SameSiteStrictMode,
		Domain:   "",
		HttpOnly: true,
		Secure:   true,
	}
}

func (j *Auth) GetExpiredRefreshCookie(refreshToken string) *http.Cookie {
	return &http.Cookie{
		Name:     j.CookieName,
		Path:     "/",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		Domain:   "",
		HttpOnly: true,
		Secure:   true,
	}
}

func (j *Auth) GetTokenFromHeaderAndVerify(c *gin.Context) (int, error) { // Changed return types
	c.Header("Vary", "Authorization")

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return 0, errors.New("Authorization header is required") // Return 0 for ID on error
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return 0, errors.New("invalid authorization header")
	}

	token := headerParts[1]
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.Secret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "Expired") {
			return 0, errors.New("Token is expired")
		}
		return 0, err
	}

	if claims.Issuer != j.Issuer {
		return 0, errors.New("Invalid issuer")
	}

	// Success: Return the UserID from the claims struct and nil error
	return claims.UserID, nil
}
