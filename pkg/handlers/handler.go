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
