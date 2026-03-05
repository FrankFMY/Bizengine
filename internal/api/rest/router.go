package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/bizengine/engine/internal/core/auth"
)

// ViewUnsubscriber cleans up view subscriptions (e.g., on logout).
type ViewUnsubscriber interface {
	UnsubscribeAll(seanceID string)
}

// RouterDeps holds all dependencies for the REST router.
type RouterDeps struct {
	AuthSvc        *auth.Service
	AuthRepo       auth.Repository
	CookieSecure   bool
	AllowedOrigins []string
	Disconnector   Disconnector
	ViewUnsub      ViewUnsubscriber
	RedisClient    *redis.Client
	Pool          *pgxpool.Pool
	EntityH       *EntityHandler
	EventH        *EventHandler
	OrganizationH *OrganizationHandler
	CatalogH      *CatalogHandler
	WarehouseH    *WarehouseHandler
	OrderH        *OrderHandler
	ProcessH      *ProcessHandler
	HRH           *HRHandler
	FinanceH      *FinanceHandler
	LogisticsH    *LogisticsHandler
	AdminH        *AdminHandler
	HealthH       *HealthHandler
}

// NewRouter creates a Chi router with all routes configured.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(Recoverer)
	r.Use(CORSMiddleware(deps.AllowedOrigins))
	r.Use(Logger)
	r.Use(TrimStrings)
	r.Use(UnwrapRequest)

	// Health check (no auth)
	if deps.HealthH != nil {
		r.Get("/health", deps.HealthH.Handle)
	} else {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respondOK(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	}

	r.Route("/api/v1", func(r chi.Router) {
		authH := NewAuthHandler(deps.AuthSvc, deps.CookieSecure, deps.Disconnector, deps.ViewUnsub)

		// Dev convenience endpoint (no auth middleware)
		if deps.Pool != nil {
			passH := NewPassTempHandler(deps.Pool, deps.AuthSvc, deps.CookieSecure)
			r.Post("/pass/temp", passH.Handle)
		}

		// Auth endpoints (no auth middleware)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)

			// Logout and unlock only require a valid session (seance may be expired)
			r.Group(func(r chi.Router) {
				r.Use(auth.SessionOnlyMiddleware(deps.AuthSvc))
				r.Post("/logout", authH.Logout)
				r.Post("/unlock", authH.Unlock)
			})

			// Switch and check require full auth (session + seance)
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(deps.AuthSvc))
				r.Post("/switch", authH.SwitchOrganization)
				r.Get("/check", authH.Check)
			})
		})

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(deps.AuthSvc))
			if deps.RedisClient != nil {
				r.Use(Idempotency(deps.RedisClient))
			}

			// Organizations
			r.Post("/organizations", deps.OrganizationH.Create)
			r.Get("/organizations", deps.OrganizationH.List)

			r.Route("/organizations/{orgID}", func(r chi.Router) {
				r.Get("/", deps.OrganizationH.Get)
				r.Put("/", deps.OrganizationH.Update)

				// Members
				r.Post("/members", deps.OrganizationH.AddMember)
				r.Get("/members", deps.OrganizationH.ListMembers)
				r.Put("/members/{userID}", deps.OrganizationH.UpdateMember)
				r.Delete("/members/{userID}", deps.OrganizationH.RemoveMember)

				// Entities
				r.Post("/entities", deps.EntityH.Create)
				r.Get("/entities", deps.EntityH.List)
				r.Get("/entities/{id}", deps.EntityH.Get)
				r.Put("/entities/{id}", deps.EntityH.Update)
				r.Delete("/entities/{id}", deps.EntityH.Delete)

				// Components
				r.Put("/entities/{entityID}/components/{type}", deps.EntityH.SetComponent)
				r.Get("/entities/{entityID}/components/{type}", deps.EntityH.GetComponent)
				r.Get("/entities/{entityID}/components", deps.EntityH.ListComponents)
				r.Delete("/entities/{entityID}/components/{type}", deps.EntityH.DeleteComponent)

				// Events
				r.Get("/events", deps.EventH.List)
				r.Get("/events/entity/{entityID}", deps.EventH.GetByEntity)

				// Catalog
				r.Route("/catalog", func(r chi.Router) {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("catalog.view"))
						r.Get("/products", deps.CatalogH.ListProducts)
						r.Get("/products/{id}", deps.CatalogH.GetProduct)
						r.Get("/categories", deps.CatalogH.ListCategories)
					})
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("catalog.manage"))
						r.Post("/products", deps.CatalogH.CreateProduct)
						r.Put("/products/{id}", deps.CatalogH.UpdateProduct)
						r.Post("/products/{id}/archive", deps.CatalogH.ArchiveProduct)
						r.Post("/categories", deps.CatalogH.CreateCategory)
						r.Put("/categories/{id}", deps.CatalogH.UpdateCategory)
						r.Delete("/categories/{id}", deps.CatalogH.DeleteCategory)
					})
				})

				// Warehouse
				r.Route("/warehouse", func(r chi.Router) {
					r.Post("/receive", deps.WarehouseH.Receive)
					r.Post("/ship", deps.WarehouseH.Ship)
					r.Post("/transfer", deps.WarehouseH.Transfer)
					r.Post("/adjust", deps.WarehouseH.Adjust)
					r.Get("/low-stock", deps.WarehouseH.GetLowStock)
					r.Get("/movements", deps.WarehouseH.ListMovements)

					r.Get("/{warehouseID}/stock", deps.WarehouseH.ListStock)
					r.Get("/{warehouseID}/stock/{productID}", deps.WarehouseH.GetStockLevel)
				})

				// Orders
				r.Route("/orders", func(r chi.Router) {
					r.Post("/", deps.OrderH.Create)
					r.Get("/", deps.OrderH.List)
					r.Get("/{id}", deps.OrderH.Get)
					r.Put("/{id}", deps.OrderH.Update)
					r.Post("/{id}/confirm", deps.OrderH.Confirm)
					r.Post("/{id}/pay", deps.OrderH.Pay)
					r.Post("/{id}/ship", deps.OrderH.Ship)
					r.Post("/{id}/deliver", deps.OrderH.Deliver)
					r.Post("/{id}/cancel", deps.OrderH.Cancel)
				})

				// Processes
				r.Route("/processes", func(r chi.Router) {
					r.Get("/definitions", deps.ProcessH.ListDefinitions)
					r.Get("/definitions/{defID}", deps.ProcessH.GetDefinition)
					r.Get("/instances", deps.ProcessH.ListInstances)
					r.Get("/instances/{instID}", deps.ProcessH.GetInstance)
					r.Get("/entity/{entityID}", deps.ProcessH.GetByEntity)
					r.Post("/trigger", deps.ProcessH.Trigger)
				})

				// HR
				r.Route("/hr", func(r chi.Router) {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("hr.view"))
						r.Get("/employees", deps.HRH.ListEmployees)
						r.Get("/employees/{id}", deps.HRH.GetEmployee)
						r.Get("/shifts", deps.HRH.ListShifts)
						r.Get("/timesheets", deps.HRH.ListTimesheets)
					})
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("hr.manage"))
						r.Post("/employees", deps.HRH.HireEmployee)
						r.Put("/employees/{id}", deps.HRH.UpdateEmployee)
						r.Post("/employees/{id}/terminate", deps.HRH.TerminateEmployee)
						r.Post("/shifts", deps.HRH.CreateShift)
						r.Put("/shifts/{id}", deps.HRH.UpdateShift)
						r.Delete("/shifts/{id}", deps.HRH.DeleteShift)
						r.Post("/timesheets/clock-in", deps.HRH.ClockIn)
						r.Post("/timesheets/{id}/clock-out", deps.HRH.ClockOut)
						r.Post("/timesheets/{id}/approve", deps.HRH.ApproveTimesheet)
					})
				})

				// Logistics
				r.Route("/logistics", func(r chi.Router) {
					r.Post("/routes", deps.LogisticsH.CreateRoute)
					r.Get("/routes", deps.LogisticsH.ListRoutes)
					r.Get("/routes/{id}", deps.LogisticsH.GetRoute)
					r.Post("/routes/{id}/start", deps.LogisticsH.StartRoute)
					r.Post("/routes/{id}/complete", deps.LogisticsH.CompleteRoute)
					r.Post("/routes/{id}/stops/{stopID}/arrive", deps.LogisticsH.ArriveAtStop)
					r.Post("/routes/{id}/stops/{stopID}/complete", deps.LogisticsH.CompleteStop)

					r.Post("/geo", deps.LogisticsH.UpdateGeo)
					r.Get("/geo/{entityID}/track", deps.LogisticsH.GetTrack)
				})

				// Admin (debug/monitoring)
				if deps.AdminH != nil {
					r.Route("/admin", func(r chi.Router) {
						r.Get("/state", deps.AdminH.State)
						r.Post("/invalidate", deps.AdminH.Invalidate)
						r.Get("/graphs", deps.AdminH.Graphs)
					})
				}

				// Finance
				r.Route("/finance", func(r chi.Router) {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("finance.view"))
						r.Get("/accounts", deps.FinanceH.ListAccounts)
						r.Get("/accounts/{id}/balance", deps.FinanceH.GetAccountBalance)
						r.Get("/transactions", deps.FinanceH.ListTransactions)
						r.Get("/transactions/{id}", deps.FinanceH.GetTransaction)
						r.Get("/invoices", deps.FinanceH.ListInvoices)
						r.Get("/reports/trial-balance", deps.FinanceH.GetTrialBalance)
					})
					r.Group(func(r chi.Router) {
						r.Use(auth.RequirePermission("finance.manage"))
						r.Post("/accounts", deps.FinanceH.CreateAccount)
						r.Post("/transactions", deps.FinanceH.CreateTransaction)
						r.Post("/transactions/{id}/post", deps.FinanceH.PostTransaction)
						r.Post("/invoices", deps.FinanceH.CreateInvoice)
						r.Post("/invoices/{id}/pay", deps.FinanceH.MarkInvoicePaid)
					})
				})
			})
		})
	})

	return r
}
