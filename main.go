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
	// .env is optional — production reads env from the container/runtime.
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file loaded (this is fine in production): %v", err)
	}
	DB := db.Connect()
	queries := db.New(DB)

	pub, err := handlers.NewZATCAPublisher()
	if err != nil {
		log.Fatal(err)
	}
	defer pub.Close()

	handlers.EnvSetup()
	h := handlers.New(DB, queries, pub)

	router := gin.Default()
	baseUrl := os.Getenv("BASEURL")
	store := persistence.NewInMemoryStore(time.Second)
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())

	// Liveness probe used by Docker HEALTHCHECK and ops tooling.
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Route-id template strings reused across REST verbs (Sonar S1192).
	const (
		routeSupplierByID     = "supplier/:id"
		routeBillByID         = "bill/:id"
		routePurchaseBillByID = "purchase_bill/:id"
		routeProductByID      = "product/:id"
		routeClientByID       = "client/:id"
	)

	authorized := router.Group(baseUrl)
	authorized.Use(handlers.JWTVerifyMiddleware)
	{
		// Convenience aliases for role-gated routes.
		admin := handlers.RequireAdmin()
		mgr := handlers.RequireManagerOrAbove()

		// ── Suppliers (employees can read/create, manager+ can edit/delete) ──
		authorized.POST("supplier/all", h.GetAllSupplier)
		authorized.GET(routeSupplierByID, h.GetSupplier)
		authorized.POST("supplier", h.AddSupplier)
		authorized.PUT(routeSupplierByID, mgr, h.EditSupplier)
		authorized.DELETE(routeSupplierByID, mgr, h.DeleteSupplier)

		// ── Companies (read for everyone, edit admin-only) ─────────────
		authorized.GET("company/all", h.GetAllCompanies)
		authorized.GET("company", h.GetCompany)
		authorized.PUT("company", admin, h.UpdateCompany)

		// ── VIN / car_part lookups (any auth) ──────────────────────────
		authorized.GET("vin/car/info/:vin", h.GetCarInfoByVin)
		authorized.GET("vin/car/:vin", h.GetCarsByVin)
		authorized.POST("vin/part/details/:vin", h.GetPartByVinDetails)
		authorized.POST("vin/part/:vin", h.GetPartByVin)
		authorized.GET("vin/car/csv/:vin", h.DownloadAllVinPartCSV)
		authorized.GET("vin/all", h.GetAllCachedVin)
		authorized.GET("vin/no_cache/:vin", h.SearchByVinSkipCache)
		authorized.GET("vin/:vin", h.SearchByVin)
		authorized.GET("car_part/:id", h.GetAllCachedVin)

		// ── Bills (delete is manager+) ─────────────────────────────────
		authorized.GET(routeBillByID, h.GetBillDetail)
		authorized.POST("bill/all", h.GetBills)
		authorized.POST("bill", h.AddBill)
		authorized.PUT(routeBillByID, h.SubmitDraftBill)
		authorized.DELETE(routeBillByID, mgr, h.DeleteBillDetail)

		authorized.GET("credit_bill/:id", h.GetBillCreditDetail)
		authorized.POST("bill/credit", h.CreditBill)

		// ── Purchase Bills (delete & receipt-tracking are manager+) ────
		authorized.GET(routePurchaseBillByID, h.GetPurchaseBillDetail)
		authorized.POST("purchase_bill", h.AddPurchaseBill)
		authorized.POST("purchase_bill/all", h.GetAllPurchaseBill)
		authorized.PUT(routePurchaseBillByID, h.UpdatePurchaseBill)
		authorized.DELETE(routePurchaseBillByID, mgr, h.DeletePurchaseBillDetail)

		// ── Products (delete is manager+) ──────────────────────────────
		authorized.POST("product/search", h.SearchProducts)
		authorized.GET(routeProductByID, h.GetProduct)
		authorized.POST("product/all", h.GetAllProducts)
		authorized.POST("product", h.AddQuantity)
		authorized.PUT(routeProductByID, h.UpdateProduct)
		authorized.DELETE(routeProductByID, mgr, h.DeleteProduct)

		// ── Clients (delete is manager+) ───────────────────────────────
		authorized.GET(routeClientByID, h.GetClient)
		authorized.POST("client/all", h.GetAllClient)
		authorized.POST("client", h.CreateClient)
		authorized.PUT(routeClientByID, h.UpdateClient)
		authorized.DELETE(routeClientByID, mgr, h.DeleteClient)

		// ── Branches (write is admin-only) ─────────────────────────────
		authorized.POST("branch/all", h.ListBranches)
		authorized.GET("branch/:id", h.GetBranch)
		authorized.POST("branch", admin, h.CreateBranch)
		authorized.PUT("branch/:id", admin, h.UpdateBranch)
		authorized.DELETE("branch/:id", admin, h.DeleteBranch)

		// ── Branch ZATCA Config (admin-only writes) ────────────────────
		authorized.GET("branch/:id/zatca", h.GetBranchZatcaConfig)
		authorized.PUT("branch/:id/zatca", admin, h.UpdateBranchZatcaConfig)
		authorized.POST("branch/:id/zatca/onboard", admin, h.OnboardBranchZatca)

		// ── Stores (write is admin-only) ───────────────────────────────
		authorized.GET("stores/all", h.GetStores)
		authorized.POST("stores/all", h.GetStores)
		authorized.GET("store/:id", h.GetStore)
		authorized.POST("store", admin, h.CreateStore)
		authorized.PUT("store/:id", admin, h.UpdateStore)
		authorized.DELETE("store/:id", admin, h.DeleteStore)

		// ── Settings (read for everyone, write admin-only) ─────────────
		authorized.GET("settings", h.GetSettings)
		authorized.PUT("settings", admin, h.UpdateSettings)

		// ── Stock / Inventory (any auth) ───────────────────────────────
		authorized.POST("stock/adjust", h.StockAdjust)
		authorized.POST("stock/check", h.StockCheck)
		authorized.GET("stock/movements/:product_id", h.GetStockMovements)
		authorized.GET("stock/enforcement", h.GetStockEnforcement)

		// ── Notifications (config writes admin-only) ───────────────────
		authorized.GET("notification", h.GetNotifications)
		authorized.GET("notification/config", h.GetNotificationConfig)
		authorized.PUT("notification/config", admin, h.UpdateNotificationConfig)
		authorized.PUT("notification/:id/read", h.MarkNotificationRead)
		authorized.PUT("notification/read-all", h.MarkAllNotificationsRead)

		// ── Dashboard (any auth, read-only) ────────────────────────────
		authorized.GET("dashboard", h.GetDashboard)
		authorized.GET("dashboard/analytics", h.GetDashboardAnalytics)
		authorized.GET("dashboard/compare", h.GetDashboardCompare)

		authorized.GET("part/type", cache.CachePage(store, time.Minute*60*24, h.GetPartType))
		authorized.POST("part/", h.GetPart)

		// ── File uploads (delete manager+) ─────────────────────────────
		authorized.POST("upload", h.UploadFile)
		authorized.GET("files/:key", h.DownloadFile)
		authorized.DELETE("files/:key", mgr, h.DeleteFile)

		// ── Orders (delete manager+) ───────────────────────────────────
		authorized.POST("order/all", h.GetOrders)
		authorized.GET("order/:id", h.GetOrder)
		authorized.POST("order", h.CreateOrder)
		authorized.PUT("order/:id", h.UpdateOrder)
		authorized.DELETE("order/:id", mgr, h.DeleteOrder)

		// ── Cash Vouchers (approve/post/delete manager+) ───────────────
		authorized.POST("cash_voucher/all", h.ListCashVouchers)
		authorized.GET("cash_voucher/summary", h.GetCashVoucherSummary)
		authorized.GET("cash_voucher/:id", h.GetCashVoucher)
		authorized.POST("cash_voucher", h.CreateCashVoucher)
		authorized.PUT("cash_voucher/:id", h.UpdateCashVoucher)
		authorized.DELETE("cash_voucher/:id", mgr, h.DeleteCashVoucher)
		authorized.POST("cash_voucher/:id/approve", mgr, h.ApproveCashVoucher)
		authorized.POST("cash_voucher/:id/post", mgr, h.PostCashVoucher)

		// ── Supplier Report ────────────────────────────────────────────
		authorized.GET("supplier/:id/report", h.GetSupplierReport)

		// ── Purchase Bill Receipt Tracking (manager+) ──────────────────
		authorized.PUT("purchase_bill/:id/received", mgr, h.MarkBillReceived)
		authorized.DELETE("purchase_bill/:id/received", mgr, h.UnmarkBillReceived)

		// ── Self profile (any auth) ────────────────────────────────────
		authorized.GET("users/me", h.GetMe)

		// ── User management (admin-only) ───────────────────────────────
		authorized.GET("user/all", admin, h.ListUsers)
		authorized.GET("user/:id", admin, h.GetUserByID)
		authorized.POST("user", admin, h.CreateUser)
		authorized.PUT("user/:id", admin, h.UpdateUser)
		authorized.DELETE("user/:id", admin, h.DeleteUser)
		authorized.POST("user/:id/password", admin, h.AdminResetUserPassword)

		// ── ZATCA submission monitor (admin only) ──────────────────────
		authorized.GET("zatca/monitor/stats", admin, h.ZatcaMonitorStats)
		authorized.GET("zatca/monitor/branches", admin, h.ZatcaMonitorBranches)
		authorized.GET("zatca/monitor/submissions", admin, h.ZatcaMonitorSubmissions)
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

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		log.Fatal("SERVER_PORT env var is required")
	}
	// Bind on all interfaces so the container's mapped port works.
	router.Run(":" + port)
	DB.Close()
}
