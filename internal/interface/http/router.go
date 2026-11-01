package http

import (
	"net/http"

	"github.com/putteror/sea-value/internal/interface/http/handler"
	"github.com/putteror/sea-value/internal/interface/http/web"
)

type Router struct {
	configHandler *handler.ConfigHandler
}

func NewRouter(configHandler *handler.ConfigHandler) *Router {
	return &Router{
		configHandler: configHandler,
	}
}

func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// 0. Static Embedded Assets (CSS, JS)
	mux.Handle("/static/", http.StripPrefix("/static/", web.GetStaticHandler()))

	// 1. Web HTML Configuration & Multi-Page Portal UI
	mux.HandleFunc("/", r.configHandler.ServeConfigPage)

	// 2. Cron Configuration JSON API
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.configHandler.GetConfigJSON(w, req)
		case http.MethodPost:
			r.configHandler.UpdateConfigJSON(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 3. Manual Sync Endpoints (Executes and writes files)
	mux.HandleFunc("/api/sync/attendance", r.configHandler.TriggerAttendanceSync)
	mux.HandleFunc("/api/sync/overtime", r.configHandler.TriggerOvertimeSync)
	mux.HandleFunc("/api/sync/help", r.configHandler.TriggerHelpSync)
	mux.HandleFunc("/api/sync/people", r.configHandler.TriggerPeopleImport)
	mux.HandleFunc("/api/people/upload", r.configHandler.UploadPeopleExcel)
	mux.HandleFunc("/api/sync/all", r.configHandler.TriggerSyncAll)
	mux.HandleFunc("/api/sync/continue", r.configHandler.TriggerContinueSync)
	mux.HandleFunc("/api/sync/export-txt", r.configHandler.ExportTXTFile)

	// 4. Test Fetch & Connection Endpoints (Preview only)
	mux.HandleFunc("/api/test/attendance", r.configHandler.TestFetchAttendance)
	mux.HandleFunc("/api/test/overtime", r.configHandler.TestFetchOvertime)
	mux.HandleFunc("/api/test/help", r.configHandler.TestFetchHelp)
	mux.HandleFunc("/api/test/connection", r.configHandler.TestConnection)

	// 5. Devices & OT Configuration Endpoints
	mux.HandleFunc("/api/devices", r.configHandler.GetDevices)
	mux.HandleFunc("/api/devices/sync", r.configHandler.SyncDevicesFromCentAccess)
	mux.HandleFunc("/api/devices/reset-watermark", r.configHandler.ResetDeviceWatermark)
	mux.HandleFunc("/api/config/devices", r.configHandler.UpdateDevicesConfig)

	// 6. Department Rules Endpoints (CRUD, Excel Import, Export, Template)
	mux.HandleFunc("/api/department-rules", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.configHandler.GetDepartmentRules(w, req)
		case http.MethodPost:
			r.configHandler.UpsertDepartmentRule(w, req)
		case http.MethodDelete:
			r.configHandler.DeleteDepartmentRule(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/department-rules/import", r.configHandler.ImportDepartmentRulesExcel)
	mux.HandleFunc("/api/department-rules/template", r.configHandler.DownloadDepartmentRulesTemplate)
	mux.HandleFunc("/api/department-rules/export", r.configHandler.ExportDepartmentRulesExcel)
}
