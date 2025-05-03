package main

import (
	"ifritah/web-service-gin/pkg/db"
	"ifritah/web-service-gin/pkg/handlers"

	"log"
	"os"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"

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

	authorized := router.Group(baseUrl)
	authorized.Use(handlers.JWTVerifyMiddleware)
	{

		authorized.GET("supplier/all", h.GetAllSupplier)
		authorized.POST("supplier", h.AddSupplier)
		authorized.PUT("supplier/:id", h.EditSupplier)
		authorized.DELETE("supplier/:id", h.DeleteSupplier)
		authorized.GET("company/all", h.GetAllCompanies)
		authorized.GET("vin/car/info/:vin", h.GetCarInfoByVin)
		authorized.GET("vin/car/:vin", h.GetCarsByVin)
		authorized.POST("vin/part/details/:vin", h.GetPartByVinDetails)
		authorized.POST("vin/part/:vin", h.GetPartByVin)
		authorized.GET("vin/car/csv/:vin", h.DownloadAllVinPartCSV)
		authorized.GET("vin/all", h.GetAllCachedVin)
		authorized.GET("vin/no_cache/:vin", h.SearchByVinSkipCache)
		authorized.GET("vin/:vin", h.SearchByVin)
		authorized.GET("car_part/:id", h.GetAllCachedVin)
		authorized.GET("notification", h.GetNotificationAll)
		authorized.POST("bill/all", h.GetBills)
		authorized.POST("bill", h.AddBill)
		authorized.GET("bill/:id", h.GetBillDetail)
		authorized.DELETE("bill/:id", h.DeleteBillDetail)

		authorized.POST("purchase_bill", h.AddPurchaseBill)
		authorized.GET("purchase_bill/:id", h.GetPurchaseBillDetail)
		authorized.DELETE("purchase_bill/:id", h.DeletePurchaseBillDetail)

		authorized.GET("stores/all", h.GetStores)
		authorized.POST("product", h.AddQuentity)
		authorized.GET("product/all", h.GetAllProducts)

		authorized.GET("part/type", cache.CachePage(store, time.Minute*60*24, h.GetPartType))
		authorized.POST("part/", h.GetPart)

		// router.GET(baseUrl + ":id", h.GetCarPartDetail)
	}
	router.POST(baseUrl+"register", h.Register)
	router.POST(baseUrl+"login", h.Login)

	router.Run("localhost:8080")
	DB.Close()
}
