package handlers

import (
	"database/sql"
	"math/big"
	"os"
	"strconv"
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

// stringToBigFloat converts a string to a big.Float
func stringToBigFloat(s string) (*big.Float, bool) {
	// Create a new big.Float
	money := new(big.Float)

	// Set the precision (optional, default is 53 bits)
	money.SetPrec(100) // Set precision to 100 bits

	// Convert the string to a big.Float
	money, success := money.SetString(s) // SetString returns the big.Float and a boolean indicating success
	return money, success
}

func zeroBigFloat() *big.Float {
	// Create a new big.Float
	zero := new(big.Float)

	// Set the precision to 100 bits
	zero.SetPrec(100)

	// Set the value to 0
	zero.SetFloat64(0)
	return zero

}

// stringToBigInt converts a string to a big.Int
func stringToBigInt(s string) (*big.Int, bool) {
	// Create a new big.Int
	money := new(big.Int)

	// Convert the string to a big.Int
	_, success := money.SetString(s, 10) // Base 10
	return money, success
}

// joinInts converts a slice of ints to a joined string without explicit loops
func joinInts(nums []int, sep string) string {
	if len(nums) == 0 {
		return ""
	}
	if len(nums) == 1 {
		return strconv.Itoa(nums[0])
	}
	return strconv.Itoa(nums[0]) + sep + joinInts(nums[1:], sep)
}
