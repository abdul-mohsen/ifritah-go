package main

import (
	"ifritah/web-service-gin/pkg/db"
	"ifritah/web-service-gin/pkg/handlers"
	"log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
  
  log.SetFlags(log.LstdFlags | log.Lshortfile)
  err := godotenv.Load()
  if err != nil {
    log.Fatalf("unable to load .env file: %e", err)
  }
  DB := db.Connect()
  h := handlers.New(DB)
  router := gin.Default()
  baseUrl := "/api/v2/"
  // router.GET(baseUrl + ":id", h.GetCarPartDetail)
  router.GET(baseUrl + "parts_provider", h.GetPartsProvider)
  router.POST(baseUrl + "parts_provider", h.PostPartsProvider)
  router.Run("localhost:8080")
  DB.Close()
}

