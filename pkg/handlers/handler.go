package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type handler struct {
	DB *sql.DB
}

type userSession struct {
	id       float64
	username string
	exp      float64
}

func New(db *sql.DB) handler {
	return handler{db}
}

func VerifyToken(c *gin.Context) (*jwt.Token, error) {
	fullTokenString := c.Request.Header.Get("Authorization")
	fmt.Println(fullTokenString)
	tokenString := strings.Split(fullTokenString, "Bearer ")[1]
	fmt.Println(tokenString)
	// Parse the token with the secret key
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		key := os.Getenv("JWT_SECRET_KEY")
		fmt.Println(key)
		fmt.Println("_0")
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(key), nil
	})
	fmt.Println(token)

	// Check for verification errors
	if err != nil {
		return nil, err
	}

	// Check if the token is valid
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Return the verified token
	return token, nil
}

func JWTVerifyMiddleware(c *gin.Context) {
	// Get the JWT token from the Authorization header
	tokenString := c.GetHeader("Authorization")
	fmt.Println(tokenString)

	// Define the secret key used to sign the token
	secretKey := os.Getenv("JWT_SECRET_KEY") // Parse the JWT token

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// Return the secret key
		return secretKey, nil
	})

	if err != nil || !token.Valid {
		fmt.Println("Error in token")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Store the decoded JWT in the context for later use
	c.Set("decoded_jwt", token.Claims)

	fmt.Println("Everything is good")
	// Continue the request processing
	c.Next()
}

func GetSessionInfo(token jwt.Token) userSession {

	claims := token.Claims.(jwt.MapClaims)
	user := userSession{
		id:       claims["userId"].(float64),
		username: claims["username"].(string),
		exp:      claims["exp"].(float64),
	}
	return user
}
