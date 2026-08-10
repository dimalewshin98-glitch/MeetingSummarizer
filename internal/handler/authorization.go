package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/repository"
	"github.com/golang-jwt/jwt/v4"
)

var ErrNamedCookieNotPresent = errors.New("http: named cookie not present")

type Claims struct {
	jwt.RegisteredClaims
	UserID int
}

const TokenExp = time.Hour * 3

func BuildJWTString(userID int, secretKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
		},
		UserID: userID,
	})
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GetUserID(tokenString string, secretKey string) int {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secretKey), nil
		})
	if err != nil {
		return -1
	}
	if !token.Valid {
		return -1
	}
	return claims.UserID
}

func AuthMiddleware(h http.Handler, repo repository.RepositoryInterface, secretKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ctx context.Context
		if r.URL.String() == "/ping" && r.Method == "GET" {
			h.ServeHTTP(w, r)
		} else {
			var tokenString string
			cookie, err := r.Cookie("token")
			if err != nil {
				if errors.Is(err, http.ErrNoCookie) {
					http.Error(w, "", http.StatusUnauthorized)
					return
				} else {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			} else {
				tokenString = cookie.Value
			}
			userID := GetUserID(tokenString, secretKey)
			switch userID {
			case 0:
				http.Error(w, "", http.StatusUnauthorized)
				return
			case -1:
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:  "token",
				Value: tokenString,
			})
			ctx = context.WithValue(r.Context(), "userID", userID)
			h.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}
