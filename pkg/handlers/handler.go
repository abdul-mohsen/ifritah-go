package handlers

import (
	"database/sql"
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
