package webui

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"inventory-tui/internal/application/service"
	"inventory-tui/internal/domain/entity"
)

// sseHub gestiona los clientes SSE conectados y hace fan-out de eventos de venta.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[chan struct{}]struct{})}
}

func (h *sseHub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *sseHub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default: // cliente ocupado — no bloquear
		}
	}
}

// SessionTracker permite que la web UI notifique al webhook qué sesión está activa.
type SessionTracker interface {
	SetActiveWebSession(id int)
}

// Server expone el servicio de inventario como aplicación web.
type Server struct {
	svc     *service.InventoryService
	engine  *gin.Engine
	hub     *sseHub
	tracker SessionTracker // nil si no hay webhook configurado
}

// SetTracker conecta el webhook para que la web UI pueda notificar la sesión activa.
func (s *Server) SetTracker(t SessionTracker) {
	s.tracker = t
}

// NotifySale notifica a todos los clientes SSE conectados que hubo una venta.
// Llamar desde el webhook de Loyverse después de procesar cada ítem.
func (s *Server) NotifySale() {
	s.hub.broadcast()
}

func NewServer(svc *service.InventoryService) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	s := &Server{svc: svc, engine: r, hub: newSSEHub()}
	s.loadTemplates()
	s.registerRoutes()
	return s
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) loadTemplates() {
	tmpl := template.Must(template.New("").ParseGlob("ui/templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("ui/templates/partials/*.html"))
	s.engine.SetHTMLTemplate(tmpl)
}

func (s *Server) registerRoutes() {
	r := s.engine

	// Páginas completas
	r.GET("/", s.handleIndex)
	r.GET("/session/:id", s.handleSession)

	// API (llamadas htmx)
	api := r.Group("/api")
	{
		api.POST("/sessions", s.handleCreateSession)
		api.DELETE("/sessions/:id", s.handleDeleteSession)
		api.GET("/sessions/:id/export", s.handleExport)
		api.POST("/sessions/:id/scan", s.handleScan)
		api.DELETE("/scans/:id", s.handleDeleteScan)
	}

	// Partials htmx
	p := r.Group("/partials")
	{
		p.GET("/records/:id", s.handleRecordsPartial)
		p.GET("/sessions", s.handleSessionsPartial)
		p.GET("/new-session-form", s.handleNewSessionForm)
	}

	// SSE — push de eventos de venta a la web UI
	r.GET("/events", s.handleEvents)
}

// --- Páginas ---

func (s *Server) handleIndex(c *gin.Context) {
	// Al volver al listado, no hay sesión web activa para el webhook.
	if s.tracker != nil {
		s.tracker.SetActiveWebSession(0)
	}
	sessions, _ := s.svc.GetSessions(c.Request.Context())
	c.HTML(http.StatusOK, "index", gin.H{"Sessions": sessions})
}

func (s *Server) handleSession(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	sessions, _ := s.svc.GetSessions(c.Request.Context())
	var session *entity.Session
	for i, s2 := range sessions {
		if s2.ID == id {
			session = &sessions[i]
			break
		}
	}
	if session == nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	// Notificar al webhook que esta sesión está activa en la web UI.
	if s.tracker != nil {
		s.tracker.SetActiveWebSession(id)
	}

	records, _ := s.svc.GetHistory(c.Request.Context(), id)
	c.HTML(http.StatusOK, "session", gin.H{
		"Session":   session,
		"SessionID": id,
		"Records":   records,
	})
}

// --- API ---

func (s *Server) handleCreateSession(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	id, err := s.svc.CreateSession(c.Request.Context(), name)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	// HX-Redirect navega a la sesión recién creada
	c.Header("HX-Redirect", fmt.Sprintf("/session/%d", id))
	c.Status(http.StatusOK)
}

func (s *Server) handleDeleteSession(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	_ = s.svc.DeleteSession(c.Request.Context(), id)
	// Retorna string vacío → htmx elimina el elemento del DOM (outerHTML swap)
	c.String(http.StatusOK, "")
}

func (s *Server) handleExport(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	sessions, _ := s.svc.GetSessions(c.Request.Context())
	name := fmt.Sprintf("sesion-%d", id)
	for _, ses := range sessions {
		if ses.ID == id {
			name = ses.Name
			break
		}
	}

	file, err := s.svc.ExportSession(c.Request.Context(), id, name)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error al exportar: %v", err)
		return
	}
	c.FileAttachment(file, file)
}

func (s *Server) handleScan(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	barcode := c.PostForm("barcode")

	// Limpieza de duplicados por escáner (mismo patrón que la TUI)
	if n := len(barcode); n > 0 && n%2 == 0 {
		if barcode[:n/2] == barcode[n/2:] {
			barcode = barcode[:n/2]
		}
	}

	p, rec, scanErr := s.svc.ScanProduct(c.Request.Context(), id, barcode)
	records, _ := s.svc.GetHistory(c.Request.Context(), id)

	c.HTML(http.StatusOK, "partials/scan-result", gin.H{
		"Product":   p,
		"Record":    rec,
		"ScanError": scanErr,
		"Barcode":   barcode,
		"Records":   records,
		"SessionID": id,
	})
}

func (s *Server) handleDeleteScan(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	_ = s.svc.DeleteScan(c.Request.Context(), id)
	c.String(http.StatusOK, "") // vacío → htmx borra la fila
}

// --- Partials htmx ---

func (s *Server) handleRecordsPartial(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	records, _ := s.svc.GetHistory(c.Request.Context(), id)
	c.HTML(http.StatusOK, "partials/records", gin.H{
		"Records":   records,
		"SessionID": id,
	})
}

func (s *Server) handleSessionsPartial(c *gin.Context) {
	sessions, _ := s.svc.GetSessions(c.Request.Context())
	c.HTML(http.StatusOK, "partials/session-cards", gin.H{"Sessions": sessions})
}

func (s *Server) handleNewSessionForm(c *gin.Context) {
	c.HTML(http.StatusOK, "partials/new-session-form", nil)
}

// handleEvents mantiene una conexión SSE abierta y envía el evento "sale"
// cada vez que el webhook de Loyverse procesa una venta.
func (s *Server) handleEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // deshabilita buffering en nginx/cloudflare

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	clientGone := c.Request.Context().Done()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			return false
		case <-ch:
			c.SSEvent("sale", "")
			return true
		case <-time.After(25 * time.Second):
			// Keepalive para que proxies no cierren la conexión inactiva.
			fmt.Fprintf(w, ": ping\n\n")
			return true
		}
	})
}
