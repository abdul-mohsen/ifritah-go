package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	CompanyID   int    `json:"company_id"`
}

func (h *handler) Register(c *gin.Context) {

	var request RegisterRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), 10)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}

	if _, err := h.DB.Exec("INSERT INTO user (company_id, username, password ) VALUES (?, ?, ?)", request.CompanyID, request.Username, hashedPassword); err != nil {
		log.Panic(err)
	}

	c.Status(http.StatusCreated)
}

type JWTConfig struct {
	JWTSecertKey      string
	SigningMethod     string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
}

var JWTSettings JWTConfig

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Id         int64  `json:"userId"`
	Username   string `json:"username"`
	Expiration int64  `json:"exp"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(username string, userid int64) (string, error) {

	// Create a new token with custom claims
	claims := Claims{
		Id:         userid,
		Username:   username,
		Expiration: time.Now().Add(JWTSettings.AccessExpiration).Unix(), // Token expiration
		RegisteredClaims: jwt.RegisteredClaims{
			// Realm:      "Access to 'hello'",
			Audience:  []string{"http://0.0.0.0:4194/hello"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(JWTSettings.AccessExpiration)), // Token expiration
			Issuer:    "softwaret",
			Subject:   "Authentication",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	return token.SignedString([]byte(JWTSettings.JWTSecertKey))
}

func GenerateRefreshToken(username string, userid int64) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"username": username,
		"userId":   userid,
		"exp":      time.Now().Add(JWTSettings.RefreshExpiration).Unix(),
	})

	return token.SignedString([]byte(JWTSettings.JWTSecertKey))
}

func checkPassword(hashedPassword []byte, password string) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
}

func (h *handler) Login(c *gin.Context) {

	var request LoginRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	var id int64
	var hashedPassword string
	if err := h.DB.QueryRow("SELECT id, password FROM user where username = ? limit 1;", request.Username).Scan(&id, &hashedPassword); err != nil {
		log.Println(request)
		c.AbortWithError(http.StatusBadRequest, err)
	}

	err := checkPassword([]byte(hashedPassword), request.Password)
	if err != nil {
		log.Panic("Invalid password", err)
	}

	accessToken, err := GenerateAccessToken(request.Username, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
		return
	}

	refreshToken, err := GenerateRefreshToken(request.Username, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func JWTVerifyMiddleware(c *gin.Context) {
	// Get the JWT token from the Authorization header
	fullTokenString := c.GetHeader("Authorization")
	split := strings.Split(fullTokenString, "Bearer ")
	if len(split) < 2 {
		c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no access token found"))
	}

	tokenString := split[1]

	// Define the secret key used to sign the token
	secretKey := []byte(JWTSettings.JWTSecertKey)
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verify the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			// Return the secret key
			return []byte(secretKey), nil
		})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusBadRequest)
		return
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {

		if !time.Unix(claims.Expiration, 0).Before(time.Now()) {
			// Store the decoded JWT in the context for later use
			c.Set("decoded_jwt", claims)

			// Continue the request processing
			c.Next()
			return
		}
	}

	log.Println("Token is invalid")
	c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("Token is invalid"))
}

func GetSessionInfo(c *gin.Context) userSession {

	claimsStr, exist := c.Get("decoded_jwt")
	if !exist {
		log.Println("Token is invalid, but how I am here")
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	claims := claimsStr.(*Claims)
	user := userSession{
		id:       claims.Id,
		username: claims.Username,
		exp:      claims.Expiration,
	}
	return user
}
