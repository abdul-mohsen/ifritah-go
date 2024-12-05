package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
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
	AccessSecretKey   string
	RefreshSecretKey  string
	SigningMethod     string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
}

var JWTSettings = JWTConfig{
	AccessSecretKey:   os.Getenv("ACCESS_SECRET_KEY"),
	RefreshSecretKey:  os.Getenv("REFRESH_SECRET_KEY"),
	SigningMethod:     "HS512",
	AccessExpiration:  time.Minute * 15,   // Access token expires in 15 minutes
	RefreshExpiration: time.Hour * 24 * 7, // Refresh token expires in 7 days
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Id         int     `json:"userId"`
	Username   string  `json:"username"`
	Expiration float64 `json:"exp"`
}

func GenerateAccessToken(username string, userid int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"aud":      "http://0.0.0.0:4194/hello",
		"sub":      "Authentication",
		"iss":      "softwaret",
		"exp":      time.Now().Add(JWTSettings.AccessExpiration).Unix(),
		"username": username,
		"userId":   userid,
	})

	return token.SignedString([]byte(JWTSettings.AccessSecretKey))
}

func GenerateRefreshToken(username string, userid int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"username": username,
		"userId":   userid,
		"exp":      time.Now().Add(JWTSettings.RefreshExpiration).Unix(),
	})

	return token.SignedString([]byte(JWTSettings.RefreshSecretKey))
}

func checkPassword(hashedPassword []byte, password string) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
}

func (h *handler) Login(c *gin.Context) {

	var request LoginRequest
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), 10)

	var id int
	var password string
	if err := h.DB.QueryRow("SELECT id, password FROM user where username = ? limit 1;", request.Username).Scan(&id, &password); err != nil {
		log.Panic(err)
	}

	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}

	err = checkPassword(hashedPassword, password)
	if err != nil {
		fmt.Println("Invalid password")
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
	tokenString := strings.Split(fullTokenString, "Bearer ")[1]
	fmt.Println(tokenString)

	// Define the secret key used to sign the token
	secretKey := []byte(JWTSettings.AccessSecretKey)
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{
		"aud": "http://0.0.0.0:4194/hello",
		"sub": "Authentication",
		"iss": "softwaret",
	},
		func(token *jwt.Token) (interface{}, error) {
			// Verify the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			// Return the secret key
			return []byte(secretKey), nil
		})

	if err != nil || !token.Valid {
		fmt.Println("Error in token")
		fmt.Println(err)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Store the decoded JWT in the context for later use
	c.Set("decoded_jwt", token.Claims)

	// Continue the request processing
	c.Next()
}

func GetSessionInfo(c *gin.Context) userSession {

	claimsStr, exist := c.Get("decoded_jwt")
	if exist == false {
		fmt.Println("hahahhah I am going places")
	}
	claims := claimsStr.(jwt.MapClaims)
	user := userSession{
		id:       claims["userId"].(float64),
		username: claims["username"].(string),
		exp:      claims["exp"].(float64),
	}
	return user
}
