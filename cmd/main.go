package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/putteror/sea-value/internal/adapter/client"
	"github.com/putteror/sea-value/internal/adapter/excel"
	"github.com/putteror/sea-value/internal/adapter/storage"
	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/service"
	appcron "github.com/putteror/sea-value/internal/cron"
	httpinterface "github.com/putteror/sea-value/internal/interface/http"
	"github.com/putteror/sea-value/internal/interface/http/handler"
)

func main() {
	log.Println("==================================================")
	log.Println("🚀 Starting Sea Value Integration Agent")
	log.Println("==================================================")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Failed to load config: %v", err)
	}

	// 2. Initialize Adapters
	apiClient := client.NewCentAccessAPIClient(cfg)
	fileStorage := storage.NewFileStorageAdapter(cfg)
	excelReader := excel.NewExcelReaderAdapter(cfg)

	// 3. Initialize Core Domain Services
	attendanceService := service.NewAttendanceSyncService(cfg, apiClient, fileStorage)
	helpService := service.NewHelpSyncService(cfg, apiClient, fileStorage)
	peopleImportService := service.NewPeopleImportService(apiClient, excelReader)
	deviceService := service.NewDeviceService(cfg, apiClient)
	diagnosticService := service.NewDiagnosticService(cfg, apiClient, deviceService)
	syncOrchestrator := service.NewSyncOrchestratorService(attendanceService, helpService, peopleImportService)

	// 4. Start Cron Scheduler
	scheduler := appcron.NewScheduler(
		cfg,
		attendanceService,
		helpService,
		peopleImportService,
	)

	if err := scheduler.Start(); err != nil {
		log.Fatalf("[FATAL] Failed to start cron scheduler: %v", err)
	}

	// 5. Initialize Web HTTP Configuration, Manual Trigger & Test Fetch Server
	configHandler := handler.NewConfigHandler(
		cfg,
		attendanceService,
		helpService,
		deviceService,
		diagnosticService,
		syncOrchestrator,
		scheduler,
	)
	router := httpinterface.NewRouter(configHandler)
	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("🌐 Web Configuration UI listening on http://localhost:%s\n", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP ERROR] %v\n", err)
		}
	}()

	// 6. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n[INFO] Shutting down Sea Value Agent gracefully...")
	scheduler.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	log.Println("[INFO] Service stopped cleanly.")
}
