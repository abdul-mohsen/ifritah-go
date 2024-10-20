package main

import (
	"ifritah/web-service-gin/pkg/db"
	"ifritah/web-service-gin/pkg/handlers"
	"log"
	"os"

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
  baseUrl := os.Getenv("BASEURL")
  // router.GET(baseUrl + ":id", h.GetCarPartDetail)
  router.GET(baseUrl + "supplier/all", h.GetAllSupplier)
  router.POST(baseUrl + "supplier", h.AddSupplier)
  router.PUT(baseUrl + "supplier:id", h.EditSupplier)
  router.DELETE(baseUrl + "supplier:id", h.DeleteSupplier)
  router.Run("localhost:8080")
  DB.Close()
}

