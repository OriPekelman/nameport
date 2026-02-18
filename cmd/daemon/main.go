package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"nameport/internal/naming"
	"nameport/internal/notify"
	"nameport/internal/portscan"
	"nameport/internal/probe"
	"nameport/internal/storage"
	"nameport/internal/tls/ca"
	"nameport/internal/tls/issuer"
	"nameport/internal/tls/policy"
	"nameport/internal/tls/trust"
)

// Service represents a discovered HTTP service
type Service struct {
	ID         string
	Name       string
	Port       int
	TargetHost string // Target IP/host (default: 127.0.0.1)
	PID        int
	ExePath    string
	Cwd        string
	Args       []string
	Group      string // Service group for visual grouping
	UseTLS     bool
	Proxy      *httputil.ReverseProxy
}

// ServiceGroup represents a group of related services for dashboard display
type ServiceGroup struct {
	Name     string     // Group name (e.g. "ollama")
	Services []*Service // Services in this group
}

// Server manages the discovery and proxying of local services
type Server struct {
	store          *storage.Store
	blacklistStore *storage.BlacklistStore
	generator      *naming.Generator
	notifyManager  *notify.Manager
	services       map[string]*Service // key = name
	mu             sync.RWMutex
	pollInterval   time.Duration
	tlsCA          *ca.CA
	tlsIssuer      *issuer.Issuer
	tlsTrustor     trust.Trustor
	tlsEnabled     bool
	httpPort       int // HTTP listen port (default 80)
	httpsPort      int // HTTPS listen port (default 443)
}

// DefaultCAStorePath is the default location for CA material.
const DefaultCAStorePath = "~/.localtls"

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}

func main() {
	// Parse flags
	storePath := storage.DefaultStorePath()
	httpPort := 80
	httpsPort := 443
	highPort := false

	// Simple arg parsing (no flag package to keep it minimal)
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--high-port", "--dev":
			highPort = true
		case "--http-port":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &httpPort)
			}
		case "--https-port":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &httpsPort)
			}
		case "--config":
			if i+1 < len(args) {
				i++
				storePath = args[i]
			}
		default:
			// Legacy: first positional arg is store path
			if !strings.HasPrefix(args[i], "--") {
				storePath = args[i]
			}
		}
	}

	if highPort {
		httpPort = 8080
		httpsPort = 8443
	}

	// Initialize store
	store, err := storage.NewStore(storePath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize blacklist store
	blacklistStore, err := storage.NewBlacklistStore(storage.DefaultBlacklistPath())
	if err != nil {
		log.Fatalf("Failed to initialize blacklist store: %v", err)
	}

	// Initialize notification manager
	notifyCfg, err := notify.LoadConfig(notify.DefaultConfigPath())
	if err != nil {
		log.Printf("Warning: failed to load notification config: %v (using defaults)", err)
		notifyCfg = notify.DefaultConfig()
	}
	notifyMgr := notify.NewManager(notifyCfg, notify.NewPlatformNotifier())

	// Create server
	srv := &Server{
		store:          store,
		blacklistStore: blacklistStore,
		generator:      naming.NewGenerator(),
		notifyManager:  notifyMgr,
		services:       make(map[string]*Service),
		pollInterval:   2 * time.Second,
		httpPort:       httpPort,
		httpsPort:      httpsPort,
	}

	// Initialize TLS CA
	caStorePath := expandHome(DefaultCAStorePath)
	tlsCA, err := ca.NewCA(caStorePath)
	if err != nil {
		log.Printf("Warning: TLS CA initialization failed: %v (HTTPS disabled)", err)
	} else if !tlsCA.IsInitialized() {
		log.Println("TLS CA not initialized. Bootstrapping new CA...")
		if err := tlsCA.Init(); err != nil {
			log.Printf("Warning: TLS CA bootstrap failed: %v (HTTPS disabled)", err)
		} else {
			log.Println("TLS CA initialized successfully.")
		}
	}

	if tlsCA != nil && tlsCA.IsInitialized() {
		srv.tlsCA = tlsCA
		srv.tlsTrustor = trust.NewPlatformTrustor()
		pol := policy.NewPolicy()
		srv.tlsIssuer = issuer.NewIssuer(tlsCA, pol)
		srv.tlsEnabled = true

		// Check if CA is trusted by the OS
		if !srv.tlsTrustor.IsInstalled(tlsCA.RootCertPEM()) {
			if srv.tlsTrustor.NeedsElevation() {
				log.Println("WARNING: Root CA is not trusted by the OS.")
				log.Println("  Run 'sudo nameport tls init' to install the CA into the system trust store.")
				log.Println("  HTTPS will work but browsers will show certificate warnings.")
			} else {
				log.Println("Installing root CA into system trust store...")
				if err := srv.tlsTrustor.Install(tlsCA.RootCertPEM()); err != nil {
					log.Printf("Warning: failed to install CA: %v", err)
					log.Println("  HTTPS will work but browsers will show certificate warnings.")
				} else {
					log.Println("Root CA installed into system trust store.")
				}
			}
		} else {
			log.Println("TLS CA is trusted by the OS.")
		}
	}

	// Load existing services into generator to avoid name collisions
	for _, record := range store.List() {
		srv.generator.GenerateName(record.ExePath, "", record.Args) // Mark name as used
		// Backfill group for records that don't have one yet
		if record.Group == "" {
			record.Group = naming.ExtractGroupFromExe(record.ExePath, record.Name)
		}
		srv.services[record.Name] = &Service{
			ID:         record.ID,
			Name:       record.Name,
			Port:       record.Port,
			TargetHost: record.EffectiveTargetHost(),
			PID:        record.PID,
			ExePath:    record.ExePath,
			Cwd:        "",
			Args:       record.Args,
			Group:      record.Group,
			UseTLS:     record.UseTLS,
			Proxy:      nil, // Will be created on first use
		}
	}

	// Start discovery loop
	go srv.discoveryLoop()

	// Setup HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRequest)
	mux.HandleFunc("/api/services", srv.handleAPIServices)
	mux.HandleFunc("/api/rename", srv.handleAPIRename)
	mux.HandleFunc("/api/blacklist", srv.handleAPIBlacklist)
	mux.HandleFunc("/api/keep", srv.handleAPIKeep)

	log.Println("nameport daemon starting...")
	log.Printf("Storage: %s", storePath)
	if highPort {
		log.Printf("Running in high-port mode (no root required)")
	}

	httpAddr := fmt.Sprintf(":%d", httpPort)
	httpsAddr := fmt.Sprintf(":%d", httpsPort)

	// HTTP server
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	// HTTPS server (if TLS is enabled)
	var httpsServer *http.Server
	if srv.tlsEnabled {
		tlsConfig := &tls.Config{
			GetCertificate: srv.tlsIssuer.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
		httpsServer = &http.Server{
			Addr:      httpsAddr,
			Handler:   srv.addForwardedProto(mux),
			TLSConfig: tlsConfig,
		}
	}

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start HTTP listener
	go func() {
		log.Printf("Listening on %s (HTTP)", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start HTTPS listener
	if httpsServer != nil {
		go func() {
			log.Printf("Listening on %s (HTTPS, dynamic certs via local CA)", httpsAddr)
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTPS server error: %v (HTTPS disabled)", err)
			}
		}()
	}

	// Show dashboard URL
	if httpPort == 80 {
		log.Println("Dashboard: http://localhost/ or https://localhost/")
	} else {
		log.Printf("Dashboard: http://localhost:%d/", httpPort)
		if srv.tlsEnabled {
			log.Printf("           https://localhost:%d/", httpsPort)
		}
	}

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if httpsServer != nil {
		httpsServer.Shutdown(shutdownCtx)
	}
	httpServer.Shutdown(shutdownCtx)

	log.Println("Daemon stopped.")
}

// addForwardedProto wraps a handler to add X-Forwarded-Proto: https
func (s *Server) addForwardedProto(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Forwarded-Proto", "https")
		next.ServeHTTP(w, r)
	})
}

// discoveryLoop continuously scans for new services
func (s *Server) discoveryLoop() {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// Run immediately on start
	s.discover()

	for range ticker.C {
		s.discover()
	}
}

// discover scans for listening ports and updates services
func (s *Server) discover() {
	listeners, err := portscan.Scan()
	if err != nil {
		log.Printf("Port scan failed: %v", err)
		return
	}

	now := time.Now()

	// Track which services we've seen this scan
	seenIDs := make(map[string]bool)
	seenNames := make(map[string]bool)

	for _, listener := range listeners {
		// Skip our own ports
		if listener.Port == s.httpPort || listener.Port == s.httpsPort {
			continue
		}

		// Skip blacklisted services
		if s.blacklistStore.IsBlacklisted(listener.ExePath, listener.Args) {
			continue
		}

		// Skip PID-blacklisted services
		if s.blacklistStore.IsBlacklistedPID(listener.PID) {
			continue
		}

		// Detect protocol (HTTP or HTTPS)
		proto := probe.DetectProtocol("127.0.0.1", listener.Port)
		if proto == probe.ProtoNone {
			continue
		}
		useTLS := proto == probe.ProtoHTTPS

		// Compute identity hash
		id := naming.ComputeIdentityHash(listener.ExePath, listener.Args)
		seenIDs[id] = true

		// Check if we already know this service
		if existing, ok := s.store.Get(id); ok {
			seenNames[existing.Name] = true

			// Update if port, PID, or active status changed
			needsSave := false
			if existing.Port != listener.Port {
				existing.Port = listener.Port
				needsSave = true
			}
			if existing.PID != listener.PID {
				existing.PID = listener.PID
				needsSave = true
			}
			if existing.UseTLS != useTLS {
				existing.UseTLS = useTLS
				needsSave = true
			}
			if !existing.IsActive {
				existing.IsActive = true
				needsSave = true
				log.Printf("Service reactivated: %s", existing.Name)
			}

			existing.LastSeen = now

			if needsSave {
				if err := s.store.Save(existing); err != nil {
					log.Printf("Failed to update service %s: %v", existing.Name, err)
				}
			}

			// Update runtime service
			s.mu.Lock()
			if svc, exists := s.services[existing.Name]; exists {
				svc.Port = listener.Port
				svc.PID = listener.PID
				svc.Cwd = listener.Cwd
				if svc.UseTLS != useTLS {
					svc.UseTLS = useTLS
					svc.Proxy = nil // Reset proxy so it gets recreated with correct scheme
				}
			}
			s.mu.Unlock()
			continue
		}

		// Generate name for new service
		name := s.generator.GenerateName(listener.ExePath, listener.Cwd, listener.Args)

		// Create record
		record := &storage.ServiceRecord{
			ID:          id,
			Name:        name,
			Port:        listener.Port,
			PID:         listener.PID,
			ExePath:     listener.ExePath,
			Args:        listener.Args,
			UserDefined: false,
			IsActive:    true,
			LastSeen:    now,
			Keep:        false,
			Group:       naming.ExtractGroupFromExe(listener.ExePath, name),
			UseTLS:      useTLS,
		}

		// Save to store
		if err := s.store.Save(record); err != nil {
			log.Printf("Failed to save service %s: %v", name, err)
			continue
		}

		// Add to runtime services
		s.mu.Lock()
		s.services[name] = &Service{
			ID:         id,
			Name:       name,
			Port:       listener.Port,
			TargetHost: "127.0.0.1",
			PID:        listener.PID,
			ExePath:    listener.ExePath,
			Cwd:        listener.Cwd,
			Args:       listener.Args,
			Group:      record.Group,
			UseTLS:     useTLS,
		}
		s.mu.Unlock()

		seenNames[name] = true
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		log.Printf("New service: %s -> %s://127.0.0.1:%d (%s)", name, scheme, listener.Port, listener.ExePath)

		if err := s.notifyManager.Notify(notify.Notification{
			Event:   notify.EventServiceDiscovered,
			Title:   "Service Discovered",
			Message: fmt.Sprintf("%s is now available on port %d", name, listener.Port),
			URL:     s.serviceURL(name),
		}); err != nil {
			log.Printf("Notification error: %v", err)
		}
	}

	// Mark services as inactive if not seen
	s.mu.Lock()
	for name, svc := range s.services {
		if !seenNames[name] {
			if record, ok := s.store.Get(svc.ID); ok && record.IsActive {
				record.IsActive = false
				record.LastSeen = now
				s.store.Save(record)
				log.Printf("Service inactive: %s", name)

				if err := s.notifyManager.Notify(notify.Notification{
					Event:   notify.EventServiceOffline,
					Title:   "Service Offline",
					Message: fmt.Sprintf("%s is no longer available", name),
					URL:     s.dashboardURL(),
				}); err != nil {
					log.Printf("Notification error: %v", err)
				}
			}
		}
	}
	s.mu.Unlock()
}

// handleRequest routes HTTP requests to the appropriate service or dashboard
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Extract host without port
	host := r.Host
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	// If accessing by IP or localhost without specific subdomain, show dashboard
	if host == "localhost" || host == "127.0.0.1" || host == "" {
		s.serveDashboard(w, r)
		return
	}

	s.mu.RLock()
	service := s.findService(host)
	s.mu.RUnlock()

	if service == nil {
		// No service found - show dashboard with message
		s.serveDashboardWithError(w, r, fmt.Sprintf("No service found for %s", host))
		return
	}

	// Create proxy on first use
	if service.Proxy == nil {
		scheme := "http"
		if service.UseTLS {
			scheme = "https"
		}
		targetURL := fmt.Sprintf("%s://%s:%d", scheme, service.TargetHost, service.Port)
		target, err := url.Parse(targetURL)
		if err != nil {
			http.Error(w, "Invalid target URL", http.StatusInternalServerError)
			return
		}

		service.Proxy = httputil.NewSingleHostReverseProxy(target)
		if service.UseTLS {
			service.Proxy.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
		// Custom error handler
		service.Proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", host, err)
			http.Error(w, fmt.Sprintf("Service %s unavailable", host), http.StatusBadGateway)
		}
	}

	// Update Host header to match the backend
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Host = fmt.Sprintf("%s:%d", service.TargetHost, service.Port)

	service.Proxy.ServeHTTP(w, r)
}

// serviceURL returns the URL for a service based on current port config and TLS status.
func (s *Server) serviceURL(name string) string {
	if s.tlsEnabled {
		if s.httpsPort == 443 {
			return fmt.Sprintf("https://%s", name)
		}
		return fmt.Sprintf("https://%s:%d", name, s.httpsPort)
	}
	if s.httpPort == 80 {
		return fmt.Sprintf("http://%s", name)
	}
	return fmt.Sprintf("http://%s:%d", name, s.httpPort)
}

// httpServiceURL returns the HTTP URL for a service (always HTTP, regardless of TLS status).
func (s *Server) httpServiceURL(name string) string {
	if s.httpPort == 80 {
		return fmt.Sprintf("http://%s", name)
	}
	return fmt.Sprintf("http://%s:%d", name, s.httpPort)
}

// dashboardURL returns the URL of the dashboard.
func (s *Server) dashboardURL() string {
	if s.httpPort == 80 {
		return "http://localhost"
	}
	return fmt.Sprintf("http://localhost:%d", s.httpPort)
}

// findService looks up a service by hostname. It first tries an exact match,
// then tries the full hostname as a service name (for subdomain-style names
// like "api.ollama.localhost" which are stored as the full name).
// Must be called with s.mu held (at least RLock).
func (s *Server) findService(host string) *Service {
	// Exact match (covers both "ollama.localhost" and "api.ollama.localhost")
	if svc, ok := s.services[host]; ok {
		return svc
	}
	return nil
}

// serveDashboard renders the admin dashboard HTML
func (s *Server) serveDashboard(w http.ResponseWriter, r *http.Request) {
	s.serveDashboardWithError(w, r, "")
}

// serviceGroup returns the effective group for a service
func serviceGroup(svc *Service) string {
	if svc.Group != "" {
		return svc.Group
	}
	return naming.ExtractGroupFromExe(svc.ExePath, svc.Name)
}

// serveDashboardWithError renders the admin dashboard with an optional error message
func (s *Server) serveDashboardWithError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	s.mu.RLock()
	services := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		services = append(services, svc)
	}
	s.mu.RUnlock()

	// Sort services by group then name for consistent display
	sort.Slice(services, func(i, j int) bool {
		gi := serviceGroup(services[i])
		gj := serviceGroup(services[j])
		if gi != gj {
			return gi < gj
		}
		return services[i].Name < services[j].Name
	})

	// Build groups for display
	groupMap := make(map[string][]*Service)
	groupOrder := make([]string, 0)
	for _, svc := range services {
		group := serviceGroup(svc)
		if _, exists := groupMap[group]; !exists {
			groupOrder = append(groupOrder, group)
		}
		groupMap[group] = append(groupMap[group], svc)
	}

	groups := make([]ServiceGroup, 0, len(groupOrder))
	for _, name := range groupOrder {
		groups = append(groups, ServiceGroup{
			Name:     name,
			Services: groupMap[name],
		})
	}

	data := struct {
		Services   []*Service
		Groups     []ServiceGroup
		ErrorMsg   string
		Hostname   string
		TLSEnabled bool
		HTTPPort   int
		HTTPSPort  int
	}{
		Services:   services,
		Groups:     groups,
		ErrorMsg:   errorMsg,
		Hostname:   r.Host,
		TLSEnabled: s.tlsEnabled,
		HTTPPort:   s.httpPort,
		HTTPSPort:  s.httpsPort,
	}

	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIServices returns JSON list of services with health status
func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	services := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		services = append(services, svc)
	}
	s.mu.RUnlock()

	// Check health of each service
	type ServiceWithHealth struct {
		*Service
		Healthy    bool   `json:"healthy"`
		StatusCode int    `json:"status_code"`
		StatusText string `json:"status_text"`
		Protocol   string `json:"protocol"`
	}

	result := make([]ServiceWithHealth, 0, len(services))
	for _, svc := range services {
		proto := "http"
		if svc.UseTLS {
			proto = "https"
		}
		swh := ServiceWithHealth{
			Service:    svc,
			Healthy:    false,
			StatusCode: 0,
			StatusText: "unknown",
			Protocol:   proto,
		}

		// Quick health check
		client := &http.Client{Timeout: 2 * time.Second}
		if svc.UseTLS {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
		targetHost := svc.TargetHost
		if targetHost == "" {
			targetHost = "127.0.0.1"
		}
		scheme := "http"
		if svc.UseTLS {
			scheme = "https"
		}
		resp, err := client.Get(fmt.Sprintf("%s://%s:%d", scheme, targetHost, svc.Port))
		if err != nil {
			swh.StatusText = "offline"
		} else {
			resp.Body.Close()
			swh.StatusCode = resp.StatusCode
			swh.StatusText = resp.Status
			// Consider healthy if status is 2xx or 3xx
			swh.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 400
		}

		result = append(result, swh)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAPIRename handles rename requests
func (s *Server) handleAPIRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure .localhost suffix
	if !strings.HasSuffix(req.NewName, ".localhost") {
		req.NewName = req.NewName + ".localhost"
	}

	// Find service by old name
	s.mu.Lock()
	defer s.mu.Unlock()

	var service *Service
	for _, svc := range s.services {
		if svc.Name == req.OldName {
			service = svc
			break
		}
	}

	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Update in store
	if err := s.store.UpdateName(service.ID, req.NewName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update in memory
	delete(s.services, service.Name)
	service.Name = req.NewName
	service.Group = naming.ExtractGroupFromExe(service.ExePath, req.NewName)
	s.services[service.Name] = service

	log.Printf("Renamed %s -> %s", req.OldName, req.NewName)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIBlacklist handles blacklist requests
func (s *Server) handleAPIBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type  string `json:"type"` // "pid", "path", "pattern"
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	entry, err := s.blacklistStore.Add(req.Type, req.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Blacklist added: [%s] %s = %s", entry.ID, entry.Type, entry.Value)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"id":      entry.ID,
		"message": fmt.Sprintf("Blacklisted %s: %s", req.Type, req.Value),
	})
}

// handleAPIKeep handles keep status updates
func (s *Server) handleAPIKeep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
		Keep bool   `json:"keep"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Find the service
	s.mu.Lock()
	var service *Service
	for _, svc := range s.services {
		if svc.Name == req.Name {
			service = svc
			break
		}
	}
	s.mu.Unlock()

	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Update in store
	if record, ok := s.store.Get(service.ID); ok {
		record.Keep = req.Keep
		if err := s.store.Save(record); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Updated keep status for %s: %v", req.Name, req.Keep)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// dashboardHTML is the admin dashboard template
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>nameport</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #fff;
            color: #333;
            line-height: 1.5;
            padding: 40px 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        header {
            text-align: center;
            margin-bottom: 40px;
        }
        header h1 {
            font-size: 2em;
            font-weight: 600;
            color: #1a1a1a;
            margin-bottom: 8px;
        }
        header p {
            color: #666;
            font-size: 0.95em;
        }
        .card {
            background: #fff;
            border: 1px solid #e0e0e0;
            box-shadow: 0 1px 3px rgba(0,0,0,0.05);
            overflow: hidden;
        }
        .card-header {
            padding: 20px 24px;
            border-bottom: 1px solid #e0e0e0;
            background: #fafafa;
        }
        .card-header h2 {
            font-size: 1.1em;
            font-weight: 600;
            color: #1a1a1a;
        }
        .table-wrapper {
            overflow-x: auto;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.85em;
            table-layout: fixed;
        }
        th {
            text-align: left;
            padding: 10px 12px;
            font-weight: 600;
            color: #555;
            font-size: 0.75em;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            border-bottom: 1px solid #e0e0e0;
            background: #fafafa;
        }
        td {
            padding: 10px 12px;
            border-bottom: 1px solid #f0f0f0;
            vertical-align: middle;
        }
        tr:hover {
            background: #fafafa;
        }
        tr.inactive {
            opacity: 0.5;
        }
        tr.group-header {
            background: #f5f7fa;
            cursor: pointer;
            user-select: none;
        }
        tr.group-header:hover {
            background: #edf0f5;
        }
        tr.group-header td {
            padding: 8px 12px;
            font-weight: 600;
            color: #444;
            font-size: 0.85em;
            border-bottom: 1px solid #e0e0e0;
        }
        .group-toggle {
            display: inline-block;
            width: 16px;
            transition: transform 0.2s;
            margin-right: 6px;
        }
        .group-toggle.collapsed {
            transform: rotate(-90deg);
        }
        tr.group-member td:first-child {
            padding-left: 32px;
        }
        .group-count {
            font-weight: normal;
            color: #888;
            font-size: 0.9em;
            margin-left: 6px;
        }
        .name-cell {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .status-dot {
            width: 10px;
            height: 10px;
            border-radius: 50%;
            flex-shrink: 0;
        }
        .status-dot.ok { background: #4caf50; }
        .status-dot.warning { background: #ff9800; }
        .status-dot.error { background: #f44336; }
        .status-dot.offline { background: #9e9e9e; }
        .service-link {
            color: #2196f3;
            text-decoration: none;
            font-weight: 500;
        }
        .service-link:hover {
            text-decoration: underline;
        }
        .service-link.inactive {
            color: #999;
        }
        .service-links {
            display: flex;
            flex-direction: column;
            gap: 2px;
        }
        .service-link-secondary {
            color: #999;
            text-decoration: none;
            font-size: 0.8em;
        }
        .service-link-secondary:hover {
            text-decoration: underline;
            color: #666;
        }
        .btn-icon {
            background: none;
            border: none;
            cursor: pointer;
            padding: 2px 4px;
            font-size: 0.85em;
            opacity: 0.5;
            transition: opacity 0.2s;
        }
        .btn-icon:hover {
            opacity: 1;
        }
        .status-badge {
            display: inline-block;
            padding: 4px 10px;
            font-size: 0.8em;
            font-weight: 500;
            border-radius: 3px;
        }
        .status-badge.ok {
            background: #e8f5e9;
            color: #2e7d32;
        }
        .status-badge.warning {
            background: #fff3e0;
            color: #ef6c00;
        }
        .status-badge.error {
            background: #ffebee;
            color: #c62828;
        }
        .status-badge.offline {
            background: #f5f5f5;
            color: #616161;
        }
        .command {
            font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
            font-size: 0.75em;
            color: #555;
            background: #f5f5f5;
            padding: 3px 6px;
            border-radius: 3px;
            max-width: 280px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            display: block;
        }
        .keep-checkbox {
            display: flex;
            align-items: center;
            gap: 6px;
            cursor: pointer;
            font-size: 0.85em;
            color: #666;
        }
        .keep-checkbox input {
            cursor: pointer;
        }
        .btn {
            padding: 4px 10px;
            border: 1px solid #ddd;
            background: #fff;
            cursor: pointer;
            font-size: 0.75em;
            font-weight: 500;
            color: #555;
            transition: all 0.2s;
            white-space: nowrap;
        }
        .btn:hover {
            background: #f5f5f5;
            border-color: #ccc;
        }
        .btn-danger {
            background: #f44336;
            color: #fff;
            border-color: #f44336;
        }
        .btn-danger:hover {
            background: #d32f2f;
            border-color: #d32f2f;
        }
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #999;
        }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            z-index: 1000;
            justify-content: center;
            align-items: center;
        }
        .modal.active {
            display: flex;
        }
        .modal-content {
            background: white;
            padding: 24px;
            width: 90%;
            max-width: 400px;
            border: 1px solid #e0e0e0;
            box-shadow: 0 4px 20px rgba(0,0,0,0.15);
        }
        .modal-content h3 {
            margin-bottom: 20px;
            font-size: 1.1em;
        }
        .form-group {
            margin-bottom: 16px;
        }
        .form-group label {
            display: block;
            margin-bottom: 6px;
            font-size: 0.85em;
            font-weight: 500;
            color: #555;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 8px 12px;
            border: 1px solid #ddd;
            font-size: 0.9em;
        }
        .form-group input:focus, .form-group select:focus {
            outline: none;
            border-color: #2196f3;
        }
        .modal-actions {
            display: flex;
            gap: 10px;
            justify-content: flex-end;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">


        <div class="card">
            <div class="card-header">
                <h2>Discovered HTTP Servers</h2>
            </div>
            {{if .Groups}}
            <div class="table-wrapper">
            <table>
                <colgroup>
                    <col style="width: 22%">
                    <col style="width: 8%">
                    <col style="width: 7%">
                    <col style="width: 7%">
                    <col style="width: 30%">
                    <col style="width: 7%">
                    <col style="width: 10%">
                </colgroup>
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Status</th>
                        <th>Port</th>
                        <th>PID</th>
                        <th>Command</th>
                        <th>Keep</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Groups}}
                    {{if gt (len .Services) 1}}
                    <tr class="group-header" onclick="toggleGroup('{{.Name}}')">
                        <td colspan="7">
                            <span class="group-toggle" id="toggle-{{.Name}}">&#9660;</span>
                            {{.Name}}
                            <span class="group-count">({{len .Services}} services)</span>
                        </td>
                    </tr>
                    {{end}}
                    {{$groupName := .Name}}
                    {{$groupSize := len .Services}}
                    {{range .Services}}
                    <tr data-name="{{.Name}}" data-group="{{$groupName}}" id="row-{{.Name}}" class="{{if gt $groupSize 1}}group-member{{end}}">
                        <td>
                            <div class="name-cell">
                                <span class="status-dot ok" title="Origin: {{if .UseTLS}}HTTPS{{else}}HTTP{{end}}"></span>
                                {{if $.TLSEnabled}}
                                <div class="service-links">
                                    {{if eq $.HTTPSPort 443}}<a href="https://{{.Name}}" class="service-link" target="_blank" id="link-{{.Name}}">&#x1f512; https://{{.Name}}</a>{{else}}<a href="https://{{.Name}}:{{$.HTTPSPort}}" class="service-link" target="_blank" id="link-{{.Name}}">&#x1f512; https://{{.Name}}:{{$.HTTPSPort}}</a>{{end}}
                                    {{if eq $.HTTPPort 80}}<a href="http://{{.Name}}" class="service-link-secondary" target="_blank">http://{{.Name}}</a>{{else}}<a href="http://{{.Name}}:{{$.HTTPPort}}" class="service-link-secondary" target="_blank">http://{{.Name}}:{{$.HTTPPort}}</a>{{end}}
                                </div>
                                {{else}}
                                {{if eq $.HTTPPort 80}}<a href="http://{{.Name}}" class="service-link" target="_blank" id="link-{{.Name}}">http://{{.Name}}</a>{{else}}<a href="http://{{.Name}}:{{$.HTTPPort}}" class="service-link" target="_blank" id="link-{{.Name}}">http://{{.Name}}:{{$.HTTPPort}}</a>{{end}}
                                {{end}}
                                <button class="btn-icon" onclick="openRenameModal('{{.Name}}')" title="Rename">Edit</button>
                            </div>
                        </td>
                        <td>
                            <span class="status-badge ok" data-name="{{.Name}}">HTTP</span>
                        </td>
                        <td>{{.Port}}</td>
                        <td>{{.PID}}</td>
                        <td><pre class="command">{{.ExePath}}</pre></td>
                        <td>
                            <label class="keep-checkbox">
                                <input type="checkbox" id="keep-{{.Name}}" onchange="toggleKeep('{{.Name}}')">
                                <span>Keep</span>
                            </label>
                        </td>
                        <td>
                            <button class="btn btn-danger" onclick="openBlacklistModal('{{.Name}}', {{.PID}}, '{{.ExePath}}')">Blacklist</button>
                        </td>
                    </tr>
                    {{end}}
                    {{end}}
                </tbody>
            </table>
            </div>
            {{else}}
            <div class="empty-state">
                <p>No services found. Start a local HTTP server to see it here.</p>
            </div>
            {{end}}
        </div>
    </div>

    <!-- Rename Modal -->
    <div id="renameModal" class="modal">
        <div class="modal-content">
            <h3>Rename Service</h3>
            <div class="form-group">
                <label>Current Name</label>
                <input type="text" id="currentName" readonly>
            </div>
            <div class="form-group">
                <label>New Name</label>
                <input type="text" id="newName" placeholder="myapp.localhost">
            </div>
            <div class="modal-actions">
                <button class="btn" onclick="closeModal('renameModal')">Cancel</button>
                <button class="btn" onclick="confirmRename()" style="background:#2196f3;color:#fff;border-color:#2196f3;">Rename</button>
            </div>
        </div>
    </div>

    <!-- Blacklist Modal -->
    <div id="blacklistModal" class="modal">
        <div class="modal-content">
            <h3>Blacklist Service</h3>
            <div class="form-group">
                <label>Blacklist Type</label>
                <select id="blacklistType"></select>
            </div>
            <div class="form-group">
                <label>Value</label>
                <input type="text" id="blacklistValue" readonly>
            </div>
            <div class="modal-actions">
                <button class="btn" onclick="closeModal('blacklistModal')">Cancel</button>
                <button class="btn btn-danger" onclick="confirmBlacklist()">Blacklist</button>
            </div>
        </div>
    </div>

    <script>
        let currentService = {};
        const keptServices = JSON.parse(localStorage.getItem('keptServices') || '[]');
        const collapsedGroups = JSON.parse(localStorage.getItem('collapsedGroups') || '[]');

        document.addEventListener('DOMContentLoaded', () => {
            keptServices.forEach(name => {
                const checkbox = document.getElementById('keep-' + name);
                if (checkbox) checkbox.checked = true;
            });
            // Restore collapsed group state
            collapsedGroups.forEach(group => {
                setGroupCollapsed(group, true);
            });
            fetchStatus();
        });

        function toggleGroup(groupName) {
            const members = document.querySelectorAll('tr.group-member[data-group="' + groupName + '"]');
            const toggle = document.getElementById('toggle-' + groupName);
            const isCollapsed = toggle && toggle.classList.contains('collapsed');

            if (isCollapsed) {
                // Expand
                setGroupCollapsed(groupName, false);
                const idx = collapsedGroups.indexOf(groupName);
                if (idx > -1) collapsedGroups.splice(idx, 1);
            } else {
                // Collapse
                setGroupCollapsed(groupName, true);
                if (!collapsedGroups.includes(groupName)) collapsedGroups.push(groupName);
            }
            localStorage.setItem('collapsedGroups', JSON.stringify(collapsedGroups));
        }

        function setGroupCollapsed(groupName, collapsed) {
            const members = document.querySelectorAll('tr.group-member[data-group="' + groupName + '"]');
            const toggle = document.getElementById('toggle-' + groupName);

            members.forEach(row => {
                row.style.display = collapsed ? 'none' : '';
            });
            if (toggle) {
                if (collapsed) {
                    toggle.classList.add('collapsed');
                } else {
                    toggle.classList.remove('collapsed');
                }
            }
        }

        function openRenameModal(name) {
            currentService.oldName = name;
            document.getElementById('currentName').value = name;
            document.getElementById('newName').value = '';
            document.getElementById('renameModal').classList.add('active');
        }

        function openBlacklistModal(name, pid, exePath) {
            currentService = { name, pid, exePath };
            document.getElementById('blacklistValue').value = pid;

            const typeSelect = document.getElementById('blacklistType');
            typeSelect.innerHTML = '';

            const options = [
                { value: 'pid', text: 'By PID (' + pid + ')' },
                { value: 'path', text: 'By Path (' + exePath.substring(0, 50) + '...)' },
                { value: 'pattern', text: 'By Pattern (regex)' }
            ];

            options.forEach(opt => {
                const option = document.createElement('option');
                option.value = opt.value;
                option.textContent = opt.text;
                typeSelect.appendChild(option);
            });

            typeSelect.onchange = function() {
                const val = typeSelect.value;
                if (val === 'pid') document.getElementById('blacklistValue').value = pid;
                if (val === 'path') document.getElementById('blacklistValue').value = exePath;
                if (val === 'pattern') document.getElementById('blacklistValue').value = '';
                document.getElementById('blacklistValue').readOnly = (val !== 'pattern');
            };

            document.getElementById('blacklistModal').classList.add('active');
        }

        function closeModal(modalId) {
            document.getElementById(modalId).classList.remove('active');
        }

        function toggleKeep(name) {
            const checkbox = document.getElementById('keep-' + name);
            const index = keptServices.indexOf(name);

            if (checkbox.checked) {
                if (index === -1) keptServices.push(name);
            } else {
                if (index > -1) keptServices.splice(index, 1);
            }

            localStorage.setItem('keptServices', JSON.stringify(keptServices));
        }

        async function confirmRename() {
            const newName = document.getElementById('newName').value;
            if (!newName) return;

            try {
                const response = await fetch('/api/rename', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        oldName: currentService.oldName,
                        newName: newName
                    })
                });

                if (response.ok) {
                    location.reload();
                } else {
                    alert('Failed to rename: ' + await response.text());
                }
            } catch (err) {
                alert('Error: ' + err.message);
            }
        }

        async function confirmBlacklist() {
            const type = document.getElementById('blacklistType').value;
            const value = document.getElementById('blacklistValue').value;

            try {
                const response = await fetch('/api/blacklist', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ type, value })
                });

                if (response.ok) {
                    location.reload();
                } else {
                    alert('Failed to blacklist: ' + await response.text());
                }
            } catch (err) {
                alert('Error: ' + err.message);
            }
        }

        document.querySelectorAll('.modal').forEach(modal => {
            modal.addEventListener('click', (e) => {
                if (e.target === modal) closeModal(modal.id);
            });
        });

        async function fetchStatus() {
            try {
                const response = await fetch('/api/services');
                const services = await response.json();
                updateServiceStatuses(services);
            } catch (err) {
                console.error('Failed to fetch service status:', err);
            }
        }

        setInterval(fetchStatus, 3000);

        function updateServiceStatuses(services) {
            const activeServices = new Map(services.map(s => [s.Name, s]));

            document.querySelectorAll('tr[data-name]').forEach(row => {
                const name = row.getAttribute('data-name');
                const service = activeServices.get(name);
                const isKept = keptServices.includes(name);

                if (!service) {
                    if (isKept) {
                        row.classList.add('inactive');
                        const link = document.getElementById('link-' + name);
                        if (link) link.classList.add('inactive');
                        updateStatus(row, 'offline', 'INACTIVE');
                    } else {
                        row.style.display = 'none';
                    }
                    return;
                }

                // Update status dot tooltip with origin protocol
                const dot = row.querySelector('.status-dot');
                if (dot && service.protocol) {
                    dot.title = 'Origin: ' + service.protocol.toUpperCase();
                }

                const code = service.status_code || 0;

                if (code >= 200 && code < 400) {
                    updateStatus(row, 'ok', code);
                } else if (code >= 400 && code < 500) {
                    updateStatus(row, 'warning', code);
                } else if (code >= 500) {
                    updateStatus(row, 'error', code);
                } else {
                    updateStatus(row, 'offline', 'OFFLINE');
                }
            });
        }

        function updateStatus(row, statusClass, text) {
            const dot = row.querySelector('.status-dot');
            const badge = row.querySelector('.status-badge');

            if (dot) {
                dot.className = 'status-dot ' + statusClass;
            }
            if (badge) {
                badge.className = 'status-badge ' + statusClass;
                badge.textContent = text;
            }
        }
    </script>
</body>
</html>
`
