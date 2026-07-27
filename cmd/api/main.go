package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	billing_http "erp-billing-service/internal/adapters/inbound/http"
	"erp-billing-service/internal/adapters/inbound/kafka"
	grpc_adapter "erp-billing-service/internal/adapters/outbound/grpc"
	outbound_http "erp-billing-service/internal/adapters/outbound/http"
	kafka_outbound "erp-billing-service/internal/adapters/outbound/kafka"
	razorpay_outbound "erp-billing-service/internal/adapters/outbound/razorpay"
	"erp-billing-service/internal/adapters/outbound/postgres"
	"erp-billing-service/internal/application"
	"erp-billing-service/internal/config"
	"erp-billing-service/internal/database"
	"erp-billing-service/internal/validation"

	shared_kafka "github.com/efs/shared-kafka"
	"github.com/gorilla/mux"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Database
	db, err := database.InitGORM(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. Run Migrations
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize subscription validation DB & start background replication
	validation.InitDB(db)
	validation.StartSync(context.Background(), db, os.Getenv("ORGANIZATION_DB_URL"))

	// 4. Initialize Kafka Producer
	kafkaCfg := shared_kafka.LoadConfigFromEnv()
	producer, err := shared_kafka.NewProducer(kafkaCfg, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

	// 5. Initialize Repositories
	invoiceRepo := postgres.NewInvoiceRepository(db)
	paymentRepo := postgres.NewPaymentRepository(db)
	auditRepo := postgres.NewAuditLogRepository(db)
	rmRepo := postgres.NewReadModelRepository(db, cfg.InventoryServiceHTTPURL)
	salesOrderRepo := postgres.NewSalesOrderRepository(db)
	salesReturnRepo := postgres.NewSalesReturnRepository(db)
	subscriptionRepo := postgres.NewSubscriptionRepository(db)
	eventPublisher := kafka_outbound.NewEventPublisher(producer)

	// Initialize Razorpay Client
	razorpayKeyID := os.Getenv("RAZORPAY_KEY_ID")
	razorpayKeySecret := os.Getenv("RAZORPAY_KEY_SECRET")
	if razorpayKeyID == "" {
		razorpayKeyID = "rzp_test_mock_id"
	}
	if razorpayKeySecret == "" {
		razorpayKeySecret = "mock_secret"
	}
	razorpayClient := razorpay_outbound.NewRazorpayClient(razorpayKeyID, razorpayKeySecret)

	// 5.5. Initialize PDF Service
	pdfStoragePath := os.Getenv("PDF_STORAGE_PATH")
	if pdfStoragePath == "" {
		pdfStoragePath = "/var/billing/pdfs" // Default path
	}
	pdfService := application.NewPDFService(pdfStoragePath)

	// 5.6 Initialize Inventory Client
	inventoryClient, err := grpc_adapter.NewInventoryClient(cfg.InventoryServiceURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to inventory service: %v", err)
		// Proceed without inventory checks? Or fail? Plan implies we should check.
		// Converting to nil client logic inside services handles nil client (skips checks).
		inventoryClient = nil
	} else {
		defer inventoryClient.Close()
		log.Println("Connected to Inventory Service")
	}

	// 5.7 Initialize Customer Client (HTTP for synchronous fallback)
	customerClient := outbound_http.NewCustomerHTTPClient(cfg.CustomerServiceURL)

	// 5.8 Initialize S3 Service
	s3Config := &application.S3Config{
		BucketName:   cfg.S3BucketName,
		Region:       cfg.S3Region,
		AccessKey:    cfg.S3AccessKey,
		SecretKey:    cfg.S3SecretKey,
		Endpoint:     cfg.S3Endpoint,
		RootFolder:   cfg.S3RootFolder,
		UploadExpiry: cfg.S3PresignedUploadExpiry,
	}
	s3Service, err := application.NewS3Service(s3Config)
	if err != nil {
		log.Fatalf("Failed to initialize S3 service: %v", err)
	}

	// 6. Initialize Services
	invoiceService := application.NewInvoiceService(invoiceRepo, rmRepo, auditRepo, eventPublisher, pdfService, s3Service, inventoryClient, customerClient, salesOrderRepo)
	paymentService := application.NewPaymentService(paymentRepo, invoiceRepo, salesOrderRepo, rmRepo, auditRepo, eventPublisher, customerClient)
	salesOrderService := application.NewSalesOrderService(salesOrderRepo, invoiceRepo, rmRepo, eventPublisher, inventoryClient, customerClient)
	salesReturnService := application.NewSalesReturnService(salesReturnRepo, salesOrderRepo, invoiceRepo, paymentRepo, rmRepo, eventPublisher, inventoryClient)
	subscriptionService := application.NewSubscriptionService(subscriptionRepo, invoiceRepo, paymentRepo, rmRepo, eventPublisher, razorpayClient)

	// 7. Initialize Kafka Consumers
	eventHandler := kafka.NewEventHandler(db, invoiceService)
	topics := []string{
		"crm.customers",
		"crm.contacts",
		"crm.addresses",
		"inventory.services",
		"inventory.parts",
		"billing.invoices",
		"items.items", // Subscribe to item events from service and parts service
		"items.service_categories", // Subscribe to service categories events
		"org.organizations",
		"workorder.workorders",
		"workorder.estimates",
		"workorder.appointments",
		"auth.users",
	}
	consumerGroup, err := shared_kafka.NewConsumerGroup(kafkaCfg, "billing-service-group", topics, eventHandler, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}
	consumerGroup.Start()
	defer consumerGroup.Stop()

	// 8. Initialize HTTP Handlers
	invoiceHandler := billing_http.NewInvoiceHandler(invoiceService)
	paymentHandler := billing_http.NewPaymentHandler(paymentService)
	salesOrderHandler := billing_http.NewSalesOrderHandler(salesOrderService)
	salesReturnHandler := billing_http.NewSalesReturnHandler(salesReturnService)
	rmHandler := billing_http.NewReadModelHandler(rmRepo)
	estimateInvoiceHandler := billing_http.NewEstimateInvoiceHandler(invoiceService)
	customerInvoiceHandler := billing_http.NewCustomerInvoiceHandler(invoiceService, paymentService)
	subscriptionHandler := billing_http.NewSubscriptionHandler(subscriptionService)
	razorpayWebhookHandler := billing_http.NewRazorpayWebhookHandler(subscriptionService, db, eventPublisher, invoiceRepo, paymentRepo)

	router := mux.NewRouter()
	api := router.PathPrefix("/api/v1").Subrouter()

	// Customer Invoice Routes
	api.HandleFunc("/customer/invoices", customerInvoiceHandler.ListCustomerInvoices).Methods("GET")
	api.HandleFunc("/customer/invoices/{id}", customerInvoiceHandler.GetCustomerInvoice).Methods("GET")
	api.HandleFunc("/customer/invoices/{id}/payments", customerInvoiceHandler.GetCustomerInvoicePayments).Methods("GET")
	api.HandleFunc("/customer/payments", customerInvoiceHandler.ListCustomerPayments).Methods("GET")
	api.HandleFunc("/customer/payments", customerInvoiceHandler.RecordCustomerPayment).Methods("POST")
	api.HandleFunc("/customer/invoices/{id}/pdf", customerInvoiceHandler.DownloadCustomerInvoicePDF).Methods("GET")

	// Subscription Routes
	api.HandleFunc("/billing/subscriptions/create", subscriptionHandler.CreateSubscription).Methods("POST")
	api.HandleFunc("/billing/subscriptions/upgrade", subscriptionHandler.UpgradeSubscription).Methods("POST")
	api.HandleFunc("/billing/subscriptions/downgrade", subscriptionHandler.DowngradeSubscription).Methods("POST")
	api.HandleFunc("/billing/subscriptions/status", subscriptionHandler.GetStatus).Methods("GET")
	api.HandleFunc("/billing/subscriptions/history", subscriptionHandler.GetHistory).Methods("GET")

	// Razorpay Webhook Route
	api.HandleFunc("/billing/webhooks/razorpay", razorpayWebhookHandler.HandleWebhook).Methods("POST")

	// Invoice Routes
	api.HandleFunc("/billing/invoices", invoiceHandler.CreateInvoice).Methods("POST")
	api.HandleFunc("/billing/invoices", invoiceHandler.ListInvoices).Methods("GET")
	api.HandleFunc("/billing/invoices/{id}", invoiceHandler.GetInvoice).Methods("GET")
	api.HandleFunc("/billing/invoices/{id}", invoiceHandler.UpdateInvoice).Methods("PUT")
	api.HandleFunc("/billing/invoices/{id}", invoiceHandler.DeleteInvoice).Methods("DELETE")
	api.HandleFunc("/billing/invoices/{id}/status", invoiceHandler.UpdateStatus).Methods("PATCH")
	api.HandleFunc("/billing/invoices/{id}/audit-logs", invoiceHandler.GetAuditLogs).Methods("GET")

	// New Invoice Workflow Routes
	api.HandleFunc("/billing/invoices/{id}/send", invoiceHandler.SendInvoice).Methods("POST")
	api.HandleFunc("/billing/invoices/{id}/pdf", invoiceHandler.DownloadInvoicePDF).Methods("GET")
	api.HandleFunc("/billing/invoices/{id}/preview", invoiceHandler.PreviewInvoicePDF).Methods("GET")

	// Estimate to Invoice Conversion Route
	api.HandleFunc("/billing/estimates/{id}/invoice", estimateInvoiceHandler.CreateInvoiceFromEstimate).Methods("POST")

	// Payment Routes
	api.HandleFunc("/billing/payments", paymentHandler.ListPayments).Methods("GET")
	api.HandleFunc("/billing/payments", paymentHandler.RecordPayment).Methods("POST")
	api.HandleFunc("/billing/payments/{id}", paymentHandler.GetPayment).Methods("GET")
	api.HandleFunc("/billing/payments/{id}/void", paymentHandler.VoidPayment).Methods("POST")
	api.HandleFunc("/billing/invoices/{id}/payments", paymentHandler.ListPaymentsByInvoice).Methods("GET")

	// Read Model Search Routes (for UI Autocomplete)
	api.HandleFunc("/billing/search/customers", rmHandler.SearchCustomers).Methods("GET")
	api.HandleFunc("/billing/search/items", rmHandler.SearchItems).Methods("GET")
	api.HandleFunc("/billing/search/contacts", rmHandler.SearchContacts).Methods("GET")

	// Work Order Routes (served from billing service's local work_order_rms replica)
	api.HandleFunc("/billing/work-orders", rmHandler.SearchWorkOrders).Methods("GET")
	api.HandleFunc("/billing/work-orders/{id}", rmHandler.GetWorkOrder).Methods("GET")

	// Sales Order Routes
	api.HandleFunc("/billing/sales-orders", salesOrderHandler.CreateSalesOrder).Methods("POST")
	api.HandleFunc("/billing/sales-orders", salesOrderHandler.ListSalesOrders).Methods("GET")
	api.HandleFunc("/billing/sales-orders/{id}", salesOrderHandler.GetSalesOrder).Methods("GET")
	api.HandleFunc("/billing/sales-orders/{id}", salesOrderHandler.UpdateSalesOrder).Methods("PUT")
	api.HandleFunc("/billing/sales-orders/{id}/confirm", salesOrderHandler.ConfirmSalesOrder).Methods("POST")
	api.HandleFunc("/billing/sales-orders/{id}/create-invoice", salesOrderHandler.CreateInvoiceFromOrder).Methods("POST")
	api.HandleFunc("/billing/sales-orders/{id}/ship", salesOrderHandler.MarkAsShipped).Methods("POST")
	api.HandleFunc("/billing/sales-orders/{id}/deliver", salesOrderHandler.MarkAsDelivered).Methods("POST")
	api.HandleFunc("/billing/sales-orders/{id}", salesOrderHandler.DeleteSalesOrder).Methods("DELETE")
	api.HandleFunc("/billing/sales-orders/{id}/cancel", salesOrderHandler.CancelSalesOrder).Methods("POST", "DELETE")

	// Sales Return Routes
	api.HandleFunc("/billing/sales-returns", salesReturnHandler.CreateSalesReturn).Methods("POST")
	api.HandleFunc("/billing/sales-returns", salesReturnHandler.ListSalesReturns).Methods("GET")
	api.HandleFunc("/billing/sales-returns/{id}", salesReturnHandler.GetSalesReturn).Methods("GET")
	api.HandleFunc("/billing/sales-orders/{id}/returns", salesReturnHandler.GetReturnsBySalesOrder).Methods("GET")
	api.HandleFunc("/billing/sales-returns/{id}/receive", salesReturnHandler.ReceiveReturn).Methods("POST")
	api.HandleFunc("/billing/sales-returns/{id}/refund", salesReturnHandler.ProcessRefund).Methods("POST")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	// 9. Start Server
	go func() {
		log.Printf("Starting HTTP server on port %s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 10. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server exited")
}
