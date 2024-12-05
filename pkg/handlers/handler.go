package handlers

import (
	"database/sql"
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
