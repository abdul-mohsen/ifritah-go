package main

import (
	"ifritah/web-service-gin/pkg/db/gen"
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
	queries := db.New(DB)

	handlers.EnvSetup()
	h := handlers.New(DB, queries)
	router := gin.Default()
	baseUrl := os.Getenv("BASEURL")
	store := persistence.NewInMemoryStore(time.Second)
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())

	authorized := router.Group(baseUrl)
	authorized.Use(handlers.JWTVerifyMiddleware)
	{
		authorized.POST("supplier/all", h.GetAllSupplier)
		authorized.GET("supplier/:id", h.GetSupplier)
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

		// Bills
		authorized.GET("bill/:id", h.GetBillDetail)
		authorized.POST("bill/all", h.GetBills)
		authorized.POST("bill", h.AddBill)
		authorized.PUT("bill/:id", h.SubmitDraftBill)
		authorized.DELETE("bill/:id", h.DeleteBillDetail)

		authorized.GET("credit_bill/:id", h.GetBillCreditDetail)
		authorized.POST("bill/credit", h.CreditBill)

		authorized.GET("purchase_bill/:id", h.GetPurchaseBillDetail)
		authorized.POST("purchase_bill", h.AddPurchaseBill)
		authorized.POST("purchase_bill/all", h.GetAllPurchaseBill)
		authorized.PUT("purchase_bill/:id", h.UpdatePurchaseBill)
		authorized.DELETE("purchase_bill/:id", h.DeletePurchaseBillDetail)

		authorized.GET("stores/all", h.GetStores)

		authorized.GET("product/:id", h.GetProduct)
		authorized.POST("product/all", h.GetAllProducts)
		authorized.POST("product", h.AddQuantity)
		authorized.PUT("product/:id", h.UpdateProduct)
		authorized.DELETE("product/:id", h.DeleteProduct)

		authorized.GET("client/:id", h.GetClient)
		authorized.POST("client/all", h.GetAllClient)
		authorized.POST("client", h.CreateClient)
		authorized.PUT("client/:id", h.UpdateClient)
		authorized.DELETE("client/:id", h.DeleteClient)

		// Settings (admin only)
		authorized.GET("settings", h.GetSettings)
		authorized.PUT("settings", h.UpdateSettings)

		// Stock / Inventory Management
		authorized.POST("stock/adjust", h.StockAdjust)
		authorized.POST("stock/check", h.StockCheck)
		authorized.GET("stock/movements/:product_id", h.GetStockMovements)
		authorized.GET("stock/enforcement", h.GetStockEnforcement)

		// Notifications
		authorized.GET("notification", h.GetNotifications)
		authorized.GET("notification/config", h.GetNotificationConfig)
		authorized.PUT("notification/config", h.UpdateNotificationConfig)
		authorized.PUT("notification/:id/read", h.MarkNotificationRead)
		authorized.PUT("notification/read-all", h.MarkAllNotificationsRead)

		authorized.GET("dashboard", h.GetDashboard)

		authorized.GET("part/type", cache.CachePage(store, time.Minute*60*24, h.GetPartType))
		authorized.POST("part/", h.GetPart)

		// Purchase Bill File Uploads (filesystem storage)
		authorized.POST("upload", h.UploadFile)
		authorized.GET("files/:key", h.DownloadFile)
		authorized.DELETE("files/:key", h.DeleteFile)
	}

	nonAuthGroup := router.Group(baseUrl)
	{
		nonAuthGroup.GET("bill/pdf/:id", h.GetBillPDF)
		nonAuthGroup.GET("bill/credit/pdf/:id", h.GetCreditBillPDF)
		nonAuthGroup.POST("register", h.Register)
		nonAuthGroup.POST("login", h.Login)
		nonAuthGroup.POST("refresh", h.Refresh)
		nonAuthGroup.POST("forgot-password", h.ForgotPassword)
		nonAuthGroup.POST("reset-password", h.ResetPassword)
	}

	router.Run("localhost:" + os.Getenv("SERVER_PORT"))
	DB.Close()
}
