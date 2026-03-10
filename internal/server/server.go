package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/interrealm-io/realm/internal/config"
	"github.com/interrealm-io/realm/internal/identity"
)

// Server is the realm's HTTP runtime — serves capability tool endpoints
// and handles incoming inter-realm requests.
type Server struct {
	cfg      *config.Config
	identity *identity.Identity
	mux      *http.ServeMux
}

// New creates a new realm server.
func New(cfg *config.Config, id *identity.Identity) *Server {
	s := &Server{
		cfg:      cfg,
		identity: id,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Start begins listening for incoming requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Network.Port)
	log.Printf("[realm] %s listening on %s (mode: %s)",
		s.cfg.Realm.ID, addr, s.cfg.Realm.Mode)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) registerRoutes() {
	// Health check always available
	s.mux.HandleFunc("/health", s.handleHealth)

	// Only register capabilities if enabled
	if !s.cfg.Capabilities.Enabled {
		return
	}

	base := s.cfg.Capabilities.BasePath

	// Built-in capability routes
	s.mux.HandleFunc(base+"/ping", s.handlePing)
	s.mux.HandleFunc(base+"/manifest", s.handleManifest)

	// Register configured tool endpoints
	for _, tool := range s.cfg.Capabilities.Tools {
		t := tool // capture loop var
		path := base + t.Path
		log.Printf("[realm] registered tool: %s %s", t.Method, path)
		s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != t.Method {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			// Placeholder — tool handlers are registered by the realm operator
			writeJSON(w, map[string]any{
				"tool":    t.Name,
				"status":  "not_implemented",
				"message": fmt.Sprintf("tool '%s' has no handler registered", t.Name),
			})
		})
	}
}

// handleHealth is a simple liveness check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

// handlePing returns realm identity — the first capability every realm exposes.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	pubKeyPEM, _ := s.identity.PublicKeyPEM()
	writeJSON(w, map[string]any{
		"realmId":   s.cfg.Realm.ID,
		"name":      s.cfg.Realm.Name,
		"mode":      s.cfg.Realm.Mode,
		"endpoint":  s.cfg.Network.Endpoint,
		"publicKey": pubKeyPEM,
		"timestamp": time.Now().Unix(),
	})
}

// handleManifest returns the full list of capabilities this realm exposes.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	tools := make([]map[string]any, 0, len(s.cfg.Capabilities.Tools))
	for _, t := range s.cfg.Capabilities.Tools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"path":        s.cfg.Capabilities.BasePath + t.Path,
			"method":      t.Method,
			"public":      t.Public,
		})
	}
	writeJSON(w, map[string]any{
		"realmId":      s.cfg.Realm.ID,
		"capabilities": tools,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
