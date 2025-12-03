package main

import "github.com/gin-gonic/gin"

func TestMain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter()
}
