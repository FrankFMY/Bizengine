// Package main is the entry point for the BizEngine server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/api/centrifugo"
	"github.com/bizengine/engine/internal/api/rest"
	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/views"
	viewDefs "github.com/bizengine/engine/internal/views/defs"
	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/internal/core/process"
	"github.com/bizengine/engine/internal/module/catalog"
	"github.com/bizengine/engine/internal/module/finance"
	"github.com/bizengine/engine/internal/module/hr"
	"github.com/bizengine/engine/internal/module/logistics"
	"github.com/bizengine/engine/internal/module/order"
	"github.com/bizengine/engine/internal/module/warehouse"
	"github.com/bizengine/engine/internal/storage/postgres"
	redisStore "github.com/bizengine/engine/internal/storage/redis"
	"github.com/bizengine/engine/pkg/config"
	"github.com/bizengine/engine/pkg/dsl"
	"github.com/bizengine/engine/pkg/types"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Setup logger
	setupLogger(cfg.LogLevel, cfg.Env)
	log.Info().Str("env", cfg.Env).Msg("starting bizengine")

	// Database
	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("connected to PostgreSQL")

	// Redis
	redisClient := redisStore.NewClient(cfg.Redis)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	log.Info().Msg("connected to Redis")

	// Session store
	sessionStore := redisStore.NewSessionStore(redisClient)

	// Core: Event Store + Bus
	eventStore := postgres.NewEventStore(pool)
	eventBus := event.NewLocalBus()

	// Core: Entity
	entityRepo := postgres.NewEntityRepo(pool)
	entitySvc := entity.NewService(entityRepo, eventStore, eventBus)

	// Core: Auth
	authRepo := postgres.NewAuthRepo(pool)
	authSvc := auth.NewService(authRepo, sessionStore, cfg.Session.SessionTTL, cfg.Session.SeanceTTL)

	// Modules: Catalog
	catalogRepo := postgres.NewCatalogRepo(pool)
	catalogSvc := catalog.NewService(entitySvc, catalogRepo, eventBus)

	// Modules: Warehouse
	warehouseRepo := postgres.NewWarehouseRepo(pool)
	warehouseSvc := warehouse.NewService(warehouseRepo, eventBus)

	// Modules: Order
	orderRepo := postgres.NewOrderRepo(pool)
	orderSvc := order.NewService(orderRepo, entitySvc, eventBus, &warehouseAdapter{svc: warehouseSvc})

	// Modules: HR
	hrRepo := postgres.NewHRRepo(pool)
	hrSvc := hr.NewService(hrRepo, entitySvc, eventBus)

	// Modules: Finance
	financeRepo := postgres.NewFinanceRepo(pool)
	financeSvc := finance.NewService(financeRepo, eventBus)

	// Modules: Logistics
	logisticsRepo := postgres.NewLogisticsRepo(pool)
	logisticsSvc := logistics.NewService(logisticsRepo, eventBus)

	// Core: Process Engine
	processRepo := postgres.NewProcessRepo(pool)
	processEngine := process.NewEngine(processRepo, eventBus)
	loadProcessDefinitions(processEngine)

	// Inter-module event subscriptions
	setupEventSubscriptions(eventBus, warehouseSvc, processEngine, financeSvc)

	// Centrifugo publisher: forward events to Centrifugo for real-time delivery
	centPub := centrifugo.NewPublisher(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)
	eventBus.SubscribeAll(event.SubscriberFunc(centPub.HandleEvent))

	// View system
	viewRegistry := views.NewRegistry()
	viewDefs.RegisterAll(viewRegistry)
	log.Info().Int("views", len(viewRegistry.All())).Msg("registered view definitions")
	viewPub := views.NewCentrifugoViewPublisher(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)
	viewManager := views.NewManager(viewRegistry, pool, viewPub)

	// Event bus → view invalidation
	eventBus.SubscribeAll(event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		changes := views.EventToChanges(ev)
		for _, ch := range changes {
			viewManager.Invalidate(ctx, ch)
		}
		return nil
	}))

	// Centrifugo connect/subscribe proxy handlers
	connectHandler := centrifugo.NewConnectHandler(sessionStore, cfg.Session.SeanceTTL)
	subscribeHandler := centrifugo.NewSubscribeHandler(sessionStore, cfg.Session.SeanceTTL)

	// REST Router
	router := rest.NewRouter(rest.RouterDeps{
		AuthSvc:      authSvc,
		AuthRepo:     authRepo,
		CookieSecure: cfg.Session.CookieSecure,
		Disconnector: centPub,
		ViewUnsub:    viewManager,
		RedisClient:  redisClient,
		EntityH:      rest.NewEntityHandler(entitySvc),
		EventH:       rest.NewEventHandler(eventStore),
		WorkspaceH: rest.NewWorkspaceHandler(authSvc, authRepo,
			financeSvc.SeedDefaultAccounts,
			func(ctx context.Context, wsID uuid.UUID) error {
				_, err := pool.Exec(ctx,
					`INSERT INTO order_number_sequences (workspace_id, last_number) VALUES ($1, 0) ON CONFLICT DO NOTHING`,
					wsID)
				return err
			},
		),
		CatalogH:   rest.NewCatalogHandler(catalogSvc),
		WarehouseH: rest.NewWarehouseHandler(warehouseSvc),
		OrderH:     rest.NewOrderHandler(orderSvc),
		ProcessH:   rest.NewProcessHandler(processEngine),
		HRH:        rest.NewHRHandler(hrSvc),
		FinanceH:   rest.NewFinanceHandler(financeSvc),
		LogisticsH: rest.NewLogisticsHandler(logisticsSvc),
		ViewsH:     rest.NewViewsHandler(viewManager),
	})

	// Wrap router with internal endpoints for Centrifugo proxy
	mux := http.NewServeMux()
	mux.Handle("/", router)
	mux.HandleFunc("POST /api/internal/centrifugo/connect", connectHandler.ServeHTTP)
	mux.HandleFunc("POST /api/internal/centrifugo/subscribe", subscribeHandler.ServeHTTP)

	// HTTP Server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("addr", addr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down")

	// 1. Stop accepting new HTTP connections
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("http server shutdown error")
	}

	// 2. Close database pool
	pool.Close()

	log.Info().Msg("server stopped")
}

func loadProcessDefinitions(engine *process.Engine) {
	files := []string{
		"processes/order_fulfillment.yaml",
		"processes/stock_replenishment.yaml",
		"processes/delivery_tracking.yaml",
		"processes/employee_onboarding.yaml",
	}
	for _, f := range files {
		def, err := dsl.ParseFile(f)
		if err != nil {
			log.Warn().Err(err).Str("file", f).Msg("failed to load process definition")
			continue
		}
		if errs := dsl.Validate(def); len(errs) > 0 {
			log.Warn().Str("file", f).Int("errors", len(errs)).Msg("process definition has validation errors")
			continue
		}
		engine.RegisterDefinition(def)
		log.Info().Str("id", def.ID).Str("trigger", def.TriggerOn).Msg("loaded process definition")
	}
}

func setupEventSubscriptions(eventBus event.Bus, warehouseSvc *warehouse.Service, processEngine *process.Engine, financeSvc *finance.Service) {
	// Warehouse reacts to order events
	eventBus.Subscribe("order.confirmed", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data struct {
			Items       []struct {
				ProductID uuid.UUID `json:"product_id"`
				Quantity  float64   `json:"quantity"`
			} `json:"items"`
			WarehouseID *uuid.UUID `json:"warehouse_id"`
		}
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			log.Error().Err(err).Msg("warehouse: failed to parse order.confirmed event")
			return nil
		}
		if data.WarehouseID == nil {
			log.Warn().Msg("warehouse: order.confirmed without warehouse_id, skipping reservation")
			return nil
		}
		for _, item := range data.Items {
			if err := warehouseSvc.Reserve(ctx, ev.WorkspaceID, warehouse.ReserveInput{
				ProductID:   item.ProductID,
				WarehouseID: *data.WarehouseID,
				Quantity:    item.Quantity,
			}); err != nil {
				log.Error().Err(err).Str("product", item.ProductID.String()).Msg("warehouse: failed to reserve stock")
			}
		}
		return nil
	}))

	eventBus.Subscribe("order.cancelled", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data struct {
			Items       []struct {
				ProductID uuid.UUID `json:"product_id"`
				Quantity  float64   `json:"quantity"`
			} `json:"items"`
			WarehouseID *uuid.UUID `json:"warehouse_id"`
		}
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			log.Error().Err(err).Msg("warehouse: failed to parse order.cancelled event")
			return nil
		}
		if data.WarehouseID == nil {
			log.Warn().Msg("warehouse: order.cancelled without warehouse_id, skipping unreserve")
			return nil
		}
		for _, item := range data.Items {
			if err := warehouseSvc.Unreserve(ctx, ev.WorkspaceID, warehouse.UnreserveInput{
				ProductID:   item.ProductID,
				WarehouseID: *data.WarehouseID,
				Quantity:    item.Quantity,
			}); err != nil {
				log.Error().Err(err).Str("product", item.ProductID.String()).Msg("warehouse: failed to unreserve stock")
			}
		}
		return nil
	}))

	// Finance auto-transactions
	eventBus.Subscribe("order.created", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		total, _ := data["total"].(float64)
		if total > 0 {
			date := ev.Timestamp.Format("2006-01-02")
			refType := "order"
			financeSvc.CreateAutoTransaction(ctx, ev.WorkspaceID, date, "Order created", "62", "90", int64(total), &refType, ev.EntityID)
		}
		return nil
	}))

	eventBus.Subscribe("order.paid", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		amount, _ := data["amount"].(float64)
		method, _ := data["method"].(string)
		if amount > 0 {
			date := ev.Timestamp.Format("2006-01-02")
			refType := "payment"
			debitCode := "51"
			if method == "cash" {
				debitCode = "50"
			}
			financeSvc.CreateAutoTransaction(ctx, ev.WorkspaceID, date, "Order payment", debitCode, "62", int64(amount), &refType, ev.EntityID)
		}
		return nil
	}))

	eventBus.Subscribe("warehouse.stock.received", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		cost, _ := data["total_cost"].(float64)
		if cost > 0 {
			date := ev.Timestamp.Format("2006-01-02")
			refType := "stock"
			financeSvc.CreateAutoTransaction(ctx, ev.WorkspaceID, date, "Stock received", "41", "60", int64(cost), &refType, ev.EntityID)
		}
		return nil
	}))

	// Process Engine handles all events for state machine advancement
	eventBus.SubscribeAll(event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		return processEngine.HandleEvent(ctx, ev)
	}))
}

// warehouseAdapter bridges order.WarehouseChecker to warehouse.Service.
type warehouseAdapter struct {
	svc *warehouse.Service
}

func (a *warehouseAdapter) CheckAvailability(ctx context.Context, wsID uuid.UUID, items []order.CheckItem) error {
	wItems := make([]warehouse.CheckItem, len(items))
	for i, item := range items {
		wItems[i] = warehouse.CheckItem{
			ProductID:   item.ProductID,
			WarehouseID: item.WarehouseID,
			Quantity:    item.Quantity,
		}
	}
	return a.svc.CheckAvailability(ctx, wsID, wItems)
}

func setupLogger(level, env string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	if env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	}
}
