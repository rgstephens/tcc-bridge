package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/stephens/tcc-bridge/internal/log"
	"github.com/stephens/tcc-bridge/internal/matter"
	"github.com/stephens/tcc-bridge/internal/storage"
	"github.com/stephens/tcc-bridge/internal/tcc"
)

// ServiceInterface defines the interface for the main service
type ServiceInterface interface {
	GetDB() *storage.DB
	GetEncryptionKey() *storage.EncryptionKey
	GetTCCClient() *tcc.Client
	GetMatterBridge() *matter.Bridge
}

// Server is the HTTP server
type Server struct {
	port    int
	service ServiceInterface
	router  *mux.Router
	hub     *Hub
}

// NewServer creates a new HTTP server
func NewServer(port int, service ServiceInterface) *Server {
	s := &Server{
		port:    port,
		service: service,
		router:  mux.NewRouter(),
		hub:     NewHub(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/status", s.handleStatus).Methods("GET")
	api.HandleFunc("/thermostat", s.handleGetThermostat).Methods("GET")
	api.HandleFunc("/thermostat/setpoint", s.handleSetSetpoint).Methods("POST")
	api.HandleFunc("/thermostat/mode", s.handleSetMode).Methods("POST")
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config/credentials", s.handleSaveCredentials).Methods("POST")
	api.HandleFunc("/config/credentials/test", s.handleTestCredentials).Methods("POST")
	api.HandleFunc("/pairing", s.handleGetPairing).Methods("GET")
	api.HandleFunc("/pairing", s.handleDecommission).Methods("DELETE")
	api.HandleFunc("/logs", s.handleGetLogs).Methods("GET")
	api.HandleFunc("/version", s.handleVersion).Methods("GET")
	api.HandleFunc("/ws", s.handleWebSocket)

	// Serve static files for frontend
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/dist")))
}

// Run starts the HTTP server
func (s *Server) Run(ctx context.Context) error {
	// Start WebSocket hub
	go s.hub.Run(ctx)

	// Start event broadcaster
	go s.broadcastEvents(ctx)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Shutdown handler
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Info("Web server listening on port %d", s.port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// broadcastEvents broadcasts Matter events to WebSocket clients
func (s *Server) broadcastEvents(ctx context.Context) {
	bridge := s.service.GetMatterBridge()
	if bridge == nil {
		return
	}

	db := s.service.GetDB()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-bridge.Events():
			s.hub.Broadcast(event)

			// Log Matter events to database
			if event.Type == matter.EventTypeMatterEvent && event.Data != nil {
				if message, ok := event.Data["message"].(string); ok {
					db.LogEvent(storage.EventSourceMatter, storage.EventTypeConnection, message, event.Data)
				}
			}
		}
	}
}

// GetHub returns the WebSocket hub
func (s *Server) GetHub() *Hub {
	return s.hub
}
