package main

import (
	"ifritah/web-service-gin/pkg/db"
	"ifritah/web-service-gin/pkg/handlers"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"log"
	"os"
	"time"

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
	handlers.EnvSetup()
	h := handlers.New(DB)
	router := gin.Default()
	baseUrl := os.Getenv("BASEURL")
	store := persistence.NewInMemoryStore(time.Second)
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

		authorized.GET("supplier/all", h.GetAllSupplier)
		authorized.POST("supplier", h.AddSupplier)
		authorized.PUT("supplier/:id", h.EditSupplier)
		authorized.DELETE("supplier/:id", h.DeleteSupplier)
		authorized.GET("company/all", h.GetAllCompanies)
		authorized.GET("vin/car/:vin", h.GetCarsByVin)
		authorized.GET("vin/part/:vin", h.GetPartByVin)
		authorized.GET("vin/all", h.GetAllCachedVin)
		authorized.GET("car_part/:id", h.GetAllCachedVin)
		authorized.GET("vin/:vin", h.SearchByVin)
		authorized.GET("notification", h.GetNotificationAll)
		authorized.POST("bill/all", h.GetBills)
		authorized.GET("stores/all", h.GetStores)
		authorized.GET("part/type", cache.CachePage(store, time.Minute*60*24, h.GetPartType))
		// router.GET(baseUrl + ":id", h.GetCarPartDetail)
	}
	router.POST(baseUrl+"register", h.Register)
	router.POST(baseUrl+"login", h.Login)

	router.Run("localhost:8080")
	DB.Close()
}
