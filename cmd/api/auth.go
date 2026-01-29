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
	jwt.RegisteredClaims
}

func (j *Auth) GenerateTokenPair(user *jwtUser) (TokenPairs, error) {
	// Create a token and set claims
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["name"] = fmt.Sprintf("%s", user.Username)
	claims["sub"] = fmt.Sprintf("%d", user.ID)
	claims["aud"] = j.Audience
	claims["iss"] = j.Issuer
	claims["iat"] = time.Now().UTC().Unix()
	claims["typ"] = "JWT"

	// Set the expiry for JWT
	claims["exp"] = time.Now().UTC().Add(j.TokenExpiry).Unix()
	
	// Create a signed token
	signedAccessToken, err := token.SignedString([]byte(j.Secret))
	if err != nil {
		return TokenPairs{}, err
	} 

	// Create a refresh token and set claims
	refreshToken := jwt.New(jwt.SigningMethodHS256)
	refreshClaims := refreshToken.Claims.(jwt.MapClaims)
	refreshClaims["sub"] = fmt.Sprint(user.ID)
	refreshClaims["iat"] = time.Now().UTC().Unix()

	// Set the expiry for refresh token
	refreshClaims["exp"] = time.Now().UTC().Add(j.RefreshTokenExpiry).Unix()

	// Create signed refresh token
	signedRefreshToken, err := refreshToken.SignedString([]byte(j.Secret))
	if err != nil {
		return TokenPairs{}, err
	}

	// Create TokenPairs and populate with signed tokens
	var tokenPairs = TokenPairs{
		Token: signedAccessToken,
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

func (j *Auth) GetTokenFromHeaderAndVerify(w http.ResponseWriter, r *http.Request) (string, *Claims, error) {
	w.Header().Add("Vary", "Authorization")

	// get auth header
	authHeader := r.Header.Get("Authorization")

	// sanity check
	if authHeader == "" {
		return "", nil, errors.New("Authorization header is required")
	}

	// split the header on spaces
	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 {
		return "", nil, errors.New("Invalid authorization header")
	}

	// check if the header is a bearer token
	if headerParts[0] != "Bearer" {
		return "", nil, errors.New("Invalid authorization header")
	}

	// get the token
	token := headerParts[1]

	// declare an empty claims
	claims := &Claims{}

	// parse the token
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(j.Secret), nil
	})

	// check if token is expired
	if err != nil {
		if strings.HasPrefix(err.Error(), "Token is expired") {
			return "", nil, errors.New("Token is expired")
		}
		return "", nil, err
	}

	// check if the issuer is valid
	if claims.Issuer != j.Issuer {
		return "", nil, errors.New("Invalid issuer")
	}

	return token, claims, nil
}
