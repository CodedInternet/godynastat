package main

import (
	"context"
	"errors"
	"github.com/asdine/storm"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/render"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

var (
	JWT_HMAC_SECRET []byte        = []byte("xWumOlRfhu+LBi2F2e1yF4FiaopQ5mr8klL4fpILnlI=")
	JWT_LIFESPAN    time.Duration = time.Hour
)

//---
// Structs
//

// Represents a local user
type User struct {
	ID       int    `storm:"increment"` // pk
	Email    string `storm:"unique"`
	Name     string
	Password string
	Admin    bool
}

// Sets the User.Password to the hashed value for the provided plain text
func (u *User) SetPassword(pass []byte) {
	hash, _ := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	u.Password = string(hash)
}

// Compares User.Password with the provided plain text.
// Returns values directly as provided by the bcrypt library for downstream processing.
func (u *User) VerifyPassword(pass []byte) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), pass)
}

//---
// Generic payloads
//---

// Login payload
type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (l *LoginPayload) Bind(r *http.Request) error {
	return nil
}

type JWTPayload struct {
	SignedToken string `json:"token"`
}

//---
// Helper functions
//

// Produce a standard format JWT token
func newJWT(sub string) (ts string, err error) {
	// Create the Claims
	now := time.Now().UTC()
	claims := jwt.StandardClaims{
		Issuer:    ENV.JWT_ISSUER,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(JWT_LIFESPAN).Unix(),
		Subject:   sub,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString(JWT_HMAC_SECRET)
}

//---
// Views
//---

// Login looks up a user, verifies password and returns response
func Login(w http.ResponseWriter, r *http.Request) {
	data := &LoginPayload{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	var user User
	if err := ENV.DB.One("Email", data.Email, &user); err != nil {
		if err == storm.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		}
		panic(err)
	}

	err := user.VerifyPassword([]byte(data.Password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			render.Render(w, r, ErrPermissionDenied(errors.New("Invalid password")))
			return
		}
		render.Render(w, r, ErrRender(err))
		return
	}

	tokenString, err := newJWT(user.Email)
	if err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}

	render.JSON(w, r, JWTPayload{tokenString})
}

// Provides a new token to the client
func JWTRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := ctx.Value("jwt").(*jwt.Token)
	claims := token.Claims.(*jwt.StandardClaims)

	tokenString, err := newJWT(claims.Subject)
	if err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}

	render.JSON(w, r, JWTPayload{tokenString})
}

//---
// Authentication middleware
//---

var (
	JWTEmpty = errors.New("Bearer token not provided")
)

func ValidateJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var tokenStr string
		var err error

		// Get token from query params
		tokenStr = r.URL.Query().Get("jwt")

		// Get token from authorization header
		if tokenStr == "" {
			bearer := r.Header.Get("Authorization")
			if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
				tokenStr = bearer[7:]
			}
		}

		// Get token from cookie
		if tokenStr == "" {
			cookie, err := r.Cookie("jwt")
			if err == nil {
				tokenStr = cookie.Value
			}
		}

		// Token is required, cya
		if tokenStr == "" {
			render.Render(w, r, ErrUnauthorized(JWTEmpty))
			return
		}

		// parse and validate le token
		token, err := jwt.ParseWithClaims(tokenStr,
			&jwt.StandardClaims{},
			func(*jwt.Token) (interface{}, error) { return JWT_HMAC_SECRET, nil })

		if err != nil {
			// well this has gone badly
			// make sure we are actually working with the correct subclass
			var jwterr *jwt.ValidationError
			jwterr = err.(*jwt.ValidationError)

			err = errors.New("Invalid token")
			switch jwterr.Errors {
			case jwt.ValidationErrorExpired:
				err = errors.New("Token has expired")
				break
			}

			render.Render(w, r, ErrUnauthorized(err))
			return
		}

		if token.Valid {
			ctx = context.WithValue(ctx, "jwt", token)

			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			render.Render(w, r, ErrUnauthorized(err))
		}
	})
}
