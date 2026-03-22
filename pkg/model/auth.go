package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	JWTSecertKey      string
	SigningMethod     string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
}

var JWTSettings JWTConfig

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type Claims struct {
	Id          int64        `json:"userId"`
	Username    string       `json:"username"`
	Role        string       `json:"role"`
	Permissions []Permission `json:"permissions"` // optional: embed or fetch on demand
	Expiration  int64        `json:"exp"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	CompanyID   int    `json:"company_id"`
}
