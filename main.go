package main

import (
	"ifritah/web-service-gin/pkg/db"
	"ifritah/web-service-gin/pkg/handlers"
	"log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
  err := godotenv.Load()
  if err != nil {
    log.Fatalf("unable to load .env file: %e", err)
  }
  DB := db.Connect()
  h := handlers.New(DB)
  router := gin.Default()
  router.GET("/hello/:id", h.GetCarPartDetail)
  router.Run("localhost:8080")
  DB.Close()
}

