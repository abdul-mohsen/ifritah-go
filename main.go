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

	// Global middleware
	// Logger middleware will write the logs to gin.DefaultWriter even if you set with GIN_MODE=release.
	// By default gin.DefaultWriter = os.Stdout
	router.Use(gin.Logger())

	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())

	// Per route middleware, you can add as many as you desire.
	// router.GET("/benchmark", MyBenchLogger(), benchEndpoint)

	// Authorization group
	// authorized := r.Group("/", AuthRequired())
	// exactly the same as:
	authorized := router.Group(baseUrl)
	// per group middleware! in this case we use the custom created
	// AuthRequired() middleware just in the "authorized" group.
	authorized.Use(handlers.JWTVerifyMiddleware)
	{

		router.GET("supplier/all", h.GetAllSupplier)
		router.POST("supplier", h.AddSupplier)
		router.PUT("supplier:id", h.EditSupplier)
		router.DELETE("supplier:id", h.DeleteSupplier)
		router.GET("company/all", h.GetAllCompanies)
		router.GET("vin/:vin", h.SearchByVin)
		router.GET("notification", h.GetNotificationAll)
		// router.GET(baseUrl + ":id", h.GetCarPartDetail)
	}
	router.POST(baseUrl+"register", h.Register)
	router.POST(baseUrl+"login", h.Login)

	router.Run("localhost:8080")
	DB.Close()
}
