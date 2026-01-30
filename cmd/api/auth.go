package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Auth struct {
	Issuer string
	Audience string
	Secret string
	TokenExpiry time.Duration
	RefreshTokenExpiry time.Duration
	CookieDomain string
	CookiePath string
	CookieName string
}

type jwtUser struct {
	ID int `json:"id"`
	Username string `json:"username"`
}

type TokenPairs struct {
	Token string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func (j *Auth) GenerateTokenPair(user *jwtUser) (TokenPairs, error) {
	// Create claims for the access token using the structured Claims struct
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
		Name: j.CookieName,
		Path: "/",
		Value: refreshToken,
		Expires: time.Now().Add(j.RefreshTokenExpiry),
		MaxAge: int(j.RefreshTokenExpiry.Seconds()),
		SameSite: http.SameSiteStrictMode,
		Domain: "",
		HttpOnly: true,
		Secure: true,
	}
}

func (j *Auth) GetExpiredRefreshCookie(refreshToken string) *http.Cookie {
	return &http.Cookie{
		Name: j.CookieName,
		Path: "/",
		Value: "",
		Expires: time.Unix(0, 0),
		MaxAge: -1,
		SameSite: http.SameSiteStrictMode,
		Domain: "",
		HttpOnly: true,
		Secure: true,
	}
}

func (j *Auth) GetTokenFromHeaderAndVerify(w http.ResponseWriter, r *http.Request) (int, error) { // Changed return types
	w.Header().Add("Vary", "Authorization")

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, errors.New("Authorization header is required") // Return 0 for ID on error
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 {
		return 0, errors.New("Invalid authorization header")
	}

	if headerParts[0] != "Bearer" {
		return 0, errors.New("Invalid authorization header")
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
		if strings.HasPrefix(err.Error(), "Token is expired") {
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
