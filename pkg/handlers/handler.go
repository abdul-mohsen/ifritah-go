package handlers

import (
	"database/sql"
	"os"
	"time"
)

type handler struct {
	DB *sql.DB
}

type userSession struct {
	id       int64
	username string
	exp      int64
}

func New(db *sql.DB) handler {
	return handler{db}
}

func EnvSetup() {

	JWTSettings = JWTConfig{
		JWTSecertKey:      os.Getenv("JWT_SECERT_KEY"),
		SigningMethod:     "HS512",
		AccessExpiration:  time.Minute * 15,   // Access token expires in 15 minutes
		RefreshExpiration: time.Hour * 24 * 7, // Refresh token expires in 7 days
	}
}
