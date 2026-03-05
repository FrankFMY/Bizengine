// Package main is the entry point for the BizEngine server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/FrankFMY/arcana"
	"github.com/bizengine/engine/internal/analytics"
	"github.com/bizengine/engine/internal/api/centrifugo"
	"github.com/bizengine/engine/internal/api/rest"
	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/internal/core/process"
	"github.com/bizengine/engine/internal/export"
	"github.com/bizengine/engine/internal/graphs"
	"github.com/bizengine/engine/internal/integration/bank"
	"github.com/bizengine/engine/internal/integration/chestnyznak"
	"github.com/bizengine/engine/internal/integration/edo"
	"github.com/bizengine/engine/internal/integration/fns"
	"github.com/bizengine/engine/internal/module/catalog"
	"github.com/bizengine/engine/internal/module/file"
	"github.com/bizengine/engine/internal/module/finance"
	"github.com/bizengine/engine/internal/documents"
	"github.com/bizengine/engine/internal/module/crm"
	"github.com/bizengine/engine/internal/module/hr"
	"github.com/bizengine/engine/internal/module/settings"
	"github.com/bizengine/engine/internal/module/logistics"
	"github.com/bizengine/engine/internal/module/order"
	"github.com/bizengine/engine/internal/module/warehouse"
	"github.com/bizengine/engine/internal/notification"
	"github.com/bizengine/engine/internal/storage/postgres"
	redisStore "github.com/bizengine/engine/internal/storage/redis"
	s3client "github.com/bizengine/engine/internal/storage/s3"
	"github.com/bizengine/engine/internal/webhook"
	"github.com/bizengine/engine/pkg/config"
	"github.com/bizengine/engine/pkg/dsl"
	"github.com/bizengine/engine/pkg/types"
)

var version = "0.1.0"

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

	// Core: Event Store + Bus (PersistentBus auto-persists all events)
	eventStore := postgres.NewEventStore(pool)
	localBus := event.NewLocalBus()
	eventBus := event.NewPersistentBus(localBus, eventStore)

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

	// Modules: CRM
	crmRepo := postgres.NewCRMRepo(pool)
	crmSvc := crm.NewService(crmRepo, entitySvc, eventBus)

	// Modules: Settings
	settingsRepo := postgres.NewSettingsRepo(pool)
	settingsSvc := settings.NewService(settingsRepo, eventBus)

	// Documents
	docRepo := postgres.NewDocumentRepo(pool)
	docSvc, err := documents.NewService(docRepo, docRepo, docRepo)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init documents service")
	}

	// Modules: Finance
	financeRepo := postgres.NewFinanceRepo(pool)
	financeSvc := finance.NewService(financeRepo, eventBus)

	// Modules: Logistics
	logisticsRepo := postgres.NewLogisticsRepo(pool)
	logisticsSvc := logistics.NewService(logisticsRepo, eventBus)

	// Storage: S3/MinIO
	s3c, err := s3client.NewClient(ctx, s3client.Config{
		Endpoint:  cfg.S3.Endpoint,
		Bucket:    cfg.S3.Bucket,
		Region:    cfg.S3.Region,
		AccessKey: cfg.S3.AccessKey,
		SecretKey: cfg.S3.SecretKey,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create S3 client")
	}
	if err := s3c.EnsureBucket(ctx); err != nil {
		log.Warn().Err(err).Msg("failed to ensure S3 bucket (MinIO may be unavailable)")
	}

	// Modules: Files
	fileRepo := postgres.NewFileRepo(pool)
	fileSvc := file.NewService(fileRepo, &s3Adapter{client: s3c})

	// Notifications (stubs for now — swap for real SMTP/SMS in production)
	emailSender := notification.NewEmailStub()
	smsSender := notification.NewSMSStub()
	pushSender := notification.NewPushStub()
	notifSvc := notification.NewService(emailSender, smsSender, pushSender)
	notifRepo := postgres.NewNotificationRepo(pool)
	notifSvc.SetRepo(notifRepo)

	// Export
	exportSvc := export.NewService()

	// Integrations (stubs — swap for real clients when API keys are configured)
	fiscalSvc := fns.NewStub()
	edoSvc := edo.NewStub()
	markingSvc := chestnyznak.NewStub()
	bankingSvc := bank.NewStub()

	// Analytics
	analyticsSvc := analytics.NewService(pool)

	// Webhooks
	webhookRepo := postgres.NewWebhookRepo(pool)
	webhookSvc := webhook.NewService(webhookRepo)

	// Core: Process Engine
	processRepo := postgres.NewProcessRepo(pool)
	processEngine := process.NewEngine(processRepo, eventBus)
	loadProcessDefinitions(processEngine)

	// Inter-module event subscriptions
	setupEventSubscriptions(eventBus, warehouseSvc, processEngine, financeSvc, orderSvc)
	setupNotificationSubscriptions(eventBus, notifSvc, authRepo)

	// Webhook dispatch: forward all events to matching webhooks
	eventBus.SubscribeAll(event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data any
		json.Unmarshal(ev.Data, &data)
		webhookSvc.Dispatch(ctx, ev.OrganizationID, ev.Type, data)
		return nil
	}))

	// Centrifugo publisher: forward events to Centrifugo for real-time delivery
	centPub := centrifugo.NewPublisher(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)
	eventBus.SubscribeAll(event.SubscriberFunc(centPub.HandleEvent))

	// Arcana reactive sync engine
	// Arcana transport appends "/api" internally, so strip it from the configured URL
	centrifugoBase := strings.TrimSuffix(cfg.Centrifugo.APIURL, "/api")
	arcanaEngine := arcana.New(arcana.Config{
		Pool: arcana.PgxQuerier(pool),
		Transport: arcana.NewCentrifugoTransport(arcana.CentrifugoConfig{
			APIURL: centrifugoBase,
			APIKey: cfg.Centrifugo.APIKey,
		}),
		AuthFunc: func(r *http.Request) (*arcana.Identity, error) {
			sessionID := cookieValue(r, "teco_session")
			seanceID := cookieValue(r, "teco_seance")
			if sessionID == "" || seanceID == "" {
				return nil, fmt.Errorf("unauthorized")
			}

			seance, err := sessionStore.GetSeance(r.Context(), seanceID)
			if err != nil {
				return nil, fmt.Errorf("unauthorized")
			}
			if seance.SessionID != sessionID {
				return nil, fmt.Errorf("unauthorized")
			}

			sess, err := sessionStore.GetSession(r.Context(), sessionID)
			if err != nil {
				return nil, fmt.Errorf("unauthorized")
			}

			sessionStore.SlideSeance(r.Context(), seanceID, cfg.Session.SeanceTTL)

			return &arcana.Identity{
				SeanceID:    seanceID,
				UserID:      sess.UserID.String(),
				WorkspaceID: sess.OrganizationID.String(),
				Role:        sess.Role,
			}, nil
		},
	})
	graphs.RegisterAll(arcanaEngine)
	if err := arcanaEngine.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start arcana engine")
	}
	defer arcanaEngine.Stop()
	log.Info().Msg("arcana engine started")

	// Event bus → Arcana invalidation (parallel to existing view invalidation)
	eventBus.SubscribeAll(event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		data := graphs.EventDataFromJSON(ev.Data)
		if ev.EntityID != nil {
			if data == nil {
				data = make(map[string]any)
			}
			data["entity_id"] = ev.EntityID.String()
		}
		changes := graphs.EventToChanges(ev.Type, data)
		for _, ch := range changes {
			arcanaEngine.Notify(ctx, ch)
		}
		return nil
	}))

	// Centrifugo connect/subscribe proxy handlers
	connectHandler := centrifugo.NewConnectHandler(sessionStore, cfg.Session.SeanceTTL)
	subscribeHandler := centrifugo.NewSubscribeHandler(sessionStore, cfg.Session.SeanceTTL)

	// REST Router
	router := rest.NewRouter(rest.RouterDeps{
		AuthSvc:        authSvc,
		AuthRepo:       authRepo,
		CookieSecure:   cfg.Session.CookieSecure,
		AllowedOrigins: cfg.Server.AllowedOrigins,
		Disconnector:   centPub,
		RedisClient:    redisClient,
		Pool:           pool,
		OnOrgCreated: []func(ctx context.Context, orgID uuid.UUID) error{
			financeSvc.SeedDefaultAccounts,
			func(ctx context.Context, orgID uuid.UUID) error {
				_, err := pool.Exec(ctx,
					`INSERT INTO order_number_sequences (organization_id, last_number) VALUES ($1, 0) ON CONFLICT DO NOTHING`,
					orgID)
				return err
			},
		},
		EntityH: rest.NewEntityHandler(entitySvc),
		EventH:  rest.NewEventHandler(eventStore),
		OrganizationH: rest.NewOrganizationHandler(authSvc, authRepo,
			financeSvc.SeedDefaultAccounts,
			func(ctx context.Context, orgID uuid.UUID) error {
				_, err := pool.Exec(ctx,
					`INSERT INTO order_number_sequences (organization_id, last_number) VALUES ($1, 0) ON CONFLICT DO NOTHING`,
					orgID)
				return err
			},
		),
		CatalogH:      rest.NewCatalogHandler(catalogSvc),
		WarehouseH:    rest.NewWarehouseHandler(warehouseSvc),
		OrderH:        rest.NewOrderHandler(orderSvc),
		ProcessH:      rest.NewProcessHandler(processEngine),
		HRH:           rest.NewHRHandler(hrSvc),
		CRMH:          rest.NewCRMHandler(crmSvc),
		SettingsH:     rest.NewSettingsHandler(settingsSvc),
		DocumentsH:    rest.NewDocumentsHandler(docSvc),
		FinanceH:      rest.NewFinanceHandler(financeSvc),
		LogisticsH:    rest.NewLogisticsHandler(logisticsSvc),
		FileH:         rest.NewFileHandler(fileSvc),
		ExportH:       rest.NewExportHandler(exportSvc),
		IntegrationH:  rest.NewIntegrationHandler(fiscalSvc, edoSvc, markingSvc, bankingSvc),
		AnalyticsH:    rest.NewAnalyticsHandler(analyticsSvc),
		WebhookH:      rest.NewWebhookHandler(webhookSvc),
		NotificationH: rest.NewNotificationHandler(notifSvc),
		AdminH:        rest.NewAdminHandler(arcanaEngine, version),
		HealthH:       rest.NewHealthHandler(pool, redisClient, arcanaEngine, cfg.Centrifugo.APIURL, version),
	})

	// Wrap router with internal endpoints for Centrifugo proxy
	mux := http.NewServeMux()
	mux.Handle("/", router)
	mux.Handle("/arcana/", http.StripPrefix("/arcana", arcanaEngine.Handler()))
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

func setupEventSubscriptions(eventBus event.Bus, warehouseSvc *warehouse.Service, processEngine *process.Engine, financeSvc *finance.Service, orderSvc *order.Service) {
	// Warehouse reacts to order events
	eventBus.Subscribe("order.confirmed", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data struct {
			Items []struct {
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
			if err := warehouseSvc.Reserve(ctx, ev.OrganizationID, warehouse.ReserveInput{
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
			Items []struct {
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
			if err := warehouseSvc.Unreserve(ctx, ev.OrganizationID, warehouse.UnreserveInput{
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
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Order created", "62", "90", int64(math.Round(total)), &refType, ev.EntityID)
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
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Order payment", debitCode, "62", int64(math.Round(amount)), &refType, ev.EntityID)
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
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Stock received", "41", "60", int64(math.Round(cost)), &refType, ev.EntityID)
		}
		return nil
	}))

	// hr.timesheet.approved → finance auto-posting (Dt 44 Selling expenses, Ct 70 Payroll)
	eventBus.Subscribe("hr.timesheet.approved", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		hoursWorked, _ := data["hours_worked"].(float64)
		hourlyRate, _ := data["hourly_rate"].(float64)
		if hoursWorked > 0 && hourlyRate > 0 {
			amount := int64(math.Round(hoursWorked * hourlyRate * 100))
			date := ev.Timestamp.Format("2006-01-02")
			refType := "timesheet"
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Timesheet approved", "44", "70", amount, &refType, ev.EntityID)
		}
		return nil
	}))

	// order.cancelled → reverse posting (Dt 90 Sales, Ct 62 Customers) to reverse initial revenue
	eventBus.Subscribe("order.cancelled", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)

		// Try to get order total from event or by querying
		var total int64
		if t, ok := data["total"].(float64); ok && t > 0 {
			total = int64(math.Round(t))
		} else if orderIDStr, ok := data["order_id"].(string); ok && orderIDStr != "" {
			oid, err := uuid.Parse(orderIDStr)
			if err != nil {
				return nil
			}
			o, err := orderSvc.Get(ctx, ev.OrganizationID, oid, false)
			if err != nil || o == nil {
				return nil
			}
			total = o.Total
		}

		if total > 0 {
			date := ev.Timestamp.Format("2006-01-02")
			refType := "reversal"
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Order cancelled — reversal", "90", "62", total, &refType, ev.EntityID)
		}
		return nil
	}))

	// order.refunded → warehouse: return stock + finance: reversal Dt 62 Kt 51
	eventBus.Subscribe("order.refunded", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data struct {
			Items []struct {
				ProductID uuid.UUID `json:"product_id"`
				Quantity  float64   `json:"quantity"`
			} `json:"items"`
			WarehouseID  *uuid.UUID `json:"warehouse_id"`
			Total        float64    `json:"total"`
			RefundMethod string     `json:"refund_method"`
		}
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			log.Error().Err(err).Msg("refund: failed to parse order.refunded event")
			return nil
		}
		// Return stock to warehouse
		if data.WarehouseID != nil {
			for _, item := range data.Items {
				refType := "refund"
				warehouseSvc.Receive(ctx, ev.OrganizationID, warehouse.ReceiveInput{
					ProductID:   item.ProductID,
					WarehouseID: *data.WarehouseID,
					Quantity:    item.Quantity,
					Reason:      "Order refund return",
				})
				_ = refType
			}
		}
		// Finance: reversal posting Dt 62 (Customers) Ct 51 (Bank) or Ct 50 (Cash)
		if data.Total > 0 {
			date := ev.Timestamp.Format("2006-01-02")
			refType := "refund"
			creditCode := "51"
			if data.RefundMethod == "cash" {
				creditCode = "50"
			}
			financeSvc.CreateAutoTransaction(ctx, ev.OrganizationID, date, "Order refund", "62", creditCode, int64(math.Round(data.Total)), &refType, ev.EntityID)
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

func (a *warehouseAdapter) CheckAvailability(ctx context.Context, orgID uuid.UUID, items []order.CheckItem) error {
	wItems := make([]warehouse.CheckItem, len(items))
	for i, item := range items {
		wItems[i] = warehouse.CheckItem{
			ProductID:   item.ProductID,
			WarehouseID: item.WarehouseID,
			Quantity:    item.Quantity,
		}
	}
	return a.svc.CheckAvailability(ctx, orgID, wItems)
}

// s3Adapter bridges s3client.Client to file.ObjectStorage interface.
type s3Adapter struct {
	client *s3client.Client
}

func (a *s3Adapter) PresignedPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	return a.client.PresignedPutURL(ctx, key, contentType, ttl)
}

func (a *s3Adapter) PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return a.client.PresignedGetURL(ctx, key, ttl)
}

func (a *s3Adapter) HeadObject(ctx context.Context, key string) (*file.ObjectMeta, error) {
	meta, err := a.client.HeadObject(ctx, key)
	if err != nil {
		return nil, err
	}
	return &file.ObjectMeta{
		ContentType:   meta.ContentType,
		ContentLength: meta.ContentLength,
		LastModified:  meta.LastModified,
	}, nil
}

func (a *s3Adapter) Delete(ctx context.Context, key string) error {
	return a.client.Delete(ctx, key)
}

func setupNotificationSubscriptions(eventBus event.Bus, notifSvc *notification.Service, authRepo auth.Repository) {
	createNotif := func(ctx context.Context, ev types.Event, title, body, severity, refType string) {
		org, err := authRepo.GetOrganization(ctx, ev.OrganizationID)
		if err != nil {
			return
		}
		n := &notification.Notification{
			ID:             uuid.New(),
			OrganizationID: ev.OrganizationID,
			UserID:         org.OwnerID,
			Title:          title,
			Body:           body,
			Severity:       severity,
			ReferenceType:  &refType,
			ReferenceID:    ev.EntityID,
			Ver:            1,
			Upd:            ev.Timestamp,
			Iat:            ev.Timestamp,
		}
		if err := notifSvc.CreateNotification(ctx, n); err != nil {
			log.Error().Err(err).Str("type", refType).Msg("notification: failed to create")
		}
	}

	eventBus.Subscribe("order.paid", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		amount, _ := data["amount"].(float64)
		createNotif(ctx, ev, "Payment received", fmt.Sprintf("Payment of %.2f received", amount/100), "info", "order")
		return nil
	}))

	eventBus.Subscribe("warehouse.stock.low", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		var data map[string]any
		json.Unmarshal(ev.Data, &data)
		product, _ := data["product_name"].(string)
		createNotif(ctx, ev, "Low stock alert", product+" is running low", "warning", "stock")
		return nil
	}))

	eventBus.Subscribe("hr.shift.created", event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		createNotif(ctx, ev, "New shift scheduled", "A new shift has been scheduled", "info", "shift")
		return nil
	}))
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
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
