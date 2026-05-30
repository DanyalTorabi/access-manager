package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server exposes HTTP handlers for the access manager.
type Server struct {
	Store store.Store
	// APIBearerToken, if non-empty, requires Authorization: Bearer <token> on /api/v1/*.
	// /health stays public. Empty means no auth on API (local dev / loopback only — document in README).
	APIBearerToken string
	// CORSAllowedOrigins lists origins permitted via Access-Control-Allow-Origin.
	// ["*"] allows any origin (default). Empty slice disables CORS headers entirely.
	CORSAllowedOrigins []string
	// Log is an optional per-server logger. When nil, the package-level logger
	// from internal/logger is used. Set this in tests to capture log output
	// per-test without mutating global state.
	Log *slog.Logger

	metrics *Metrics
}

// serverLogger returns s.Log when set, or the package-level logger otherwise.
func (s *Server) serverLogger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return logger.Get()
}

// auditLog emits a structured audit event at INFO level with audit=true.
func (s *Server) auditLog(ctx context.Context, action string, attrs ...slog.Attr) {
	all := make([]slog.Attr, 0, len(attrs)+2)
	all = append(all, slog.Bool("audit", true), slog.String("action", action))
	all = append(all, attrs...)
	s.serverLogger().LogAttrs(ctx, slog.LevelInfo, "audit", all...)
}

// NegativeMaskCounter returns the store_negative_mask_observed_total
// Prometheus counter, or nil when metrics are disabled. It is the narrow
// accessor used by cmd/server to wire the SQLite store's negative-mask
// hook without exposing the full Metrics struct.
func (s *Server) NegativeMaskCounter() prometheus.Counter {
	if s.metrics == nil {
		return nil
	}
	return s.metrics.NegativeMaskTotal
}

// Router builds the chi router. reg and gather supply the Prometheus registry
// for metrics middleware and the /metrics endpoint. Pass nil for both to
// disable instrumentation (e.g. in tests that don't care about metrics).
func (s *Server) Router(reg prometheus.Registerer, gather prometheus.Gatherer) chi.Router {
	r := chi.NewRouter()

	// CORS is the outermost middleware so that OPTIONS preflight requests are
	// answered before Bearer auth, metrics, or handler logic runs.
	if len(s.CORSAllowedOrigins) > 0 {
		r.Use(CORSMiddleware(s.CORSAllowedOrigins))
	}

	if reg != nil {
		s.metrics = NewMetrics(reg)
		r.Use(s.metrics.Middleware)
	} else {
		s.metrics = nil
	}

	r.Get("/health", s.health)
	// /metrics is outside bearer auth so Prometheus can scrape without a token.
	// Bind to loopback or use network policy when exposing beyond localhost.
	if gather != nil {
		r.Handle("/metrics", promhttp.HandlerFor(gather, promhttp.HandlerOpts{}))
	}

	r.Route("/api/v1", func(r chi.Router) {
		if tok := strings.TrimSpace(s.APIBearerToken); tok != "" {
			r.Use(BearerAuth(tok))
		}
		r.Post("/domains", s.domainCreate)
		r.Get("/domains", s.domainList)
		r.Get("/domains/{domainID}", s.domainGet)
		r.Patch("/domains/{domainID}", s.domainPatch)
		r.Delete("/domains/{domainID}", s.domainDelete)

		r.Get("/domains/{domainID}/users", s.userList)
		r.Post("/domains/{domainID}/users", s.userCreate)
		r.Get("/domains/{domainID}/users/{userID}", s.userGet)
		r.Patch("/domains/{domainID}/users/{userID}", s.userPatch)
		r.Delete("/domains/{domainID}/users/{userID}", s.userDelete)

		r.Get("/domains/{domainID}/groups", s.groupList)
		r.Post("/domains/{domainID}/groups", s.groupCreate)
		r.Get("/domains/{domainID}/groups/{groupID}", s.groupGet)
		r.Patch("/domains/{domainID}/groups/{groupID}", s.groupPatch)
		r.Delete("/domains/{domainID}/groups/{groupID}", s.groupDelete)
		r.Patch("/domains/{domainID}/groups/{groupID}/parent", s.groupSetParent)

		r.Get("/domains/{domainID}/resources", s.resourceList)
		r.Post("/domains/{domainID}/resources", s.resourceCreate)
		r.Get("/domains/{domainID}/resources/{resourceID}", s.resourceGet)
		r.Patch("/domains/{domainID}/resources/{resourceID}", s.resourcePatch)
		r.Delete("/domains/{domainID}/resources/{resourceID}", s.resourceDelete)

		r.Get("/domains/{domainID}/access-types", s.accessTypeList)
		r.Post("/domains/{domainID}/access-types", s.accessTypeCreate)
		r.Get("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypeGet)
		r.Patch("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypePatch)
		r.Delete("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypeDelete)

		r.Get("/domains/{domainID}/permissions", s.permissionList)
		r.Post("/domains/{domainID}/permissions", s.permissionCreate)
		r.Get("/domains/{domainID}/permissions/{permissionID}", s.permissionGet)
		r.Patch("/domains/{domainID}/permissions/{permissionID}", s.permissionPatch)
		r.Delete("/domains/{domainID}/permissions/{permissionID}", s.permissionDelete)

		r.Post("/domains/{domainID}/users/{userID}/groups/{groupID}", s.addUserToGroup)
		r.Delete("/domains/{domainID}/users/{userID}/groups/{groupID}", s.removeUserFromGroup)

		r.Post("/domains/{domainID}/users/{userID}/permissions/{permissionID}", s.grantUserPermission)
		r.Delete("/domains/{domainID}/users/{userID}/permissions/{permissionID}", s.revokeUserPermission)
		r.Get("/domains/{domainID}/users/{userID}/authz/resources", s.userAuthzResources)

		r.Post("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.grantGroupPermission)
		r.Delete("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.revokeGroupPermission)
		r.Get("/domains/{domainID}/groups/{groupID}/authz/resources", s.groupAuthzResources)

		r.Get("/domains/{domainID}/resources/{resourceID}/authz/users", s.resourceAuthzUsers)
		r.Get("/domains/{domainID}/resources/{resourceID}/authz/groups", s.resourceAuthzGroups)

		r.Get("/domains/{domainID}/authz/check", s.authzCheck)
		r.Get("/domains/{domainID}/authz/masks", s.authzMasks)
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

type titleBody struct {
	Title string `json:"title"`
}

type patchTitleBody struct {
	Title *string `json:"title"`
}

// parentGroupAuditAttrs, groupPatchBody, parentBody are in server_groups.go.
// accessTypeBody, accessTypePatchBody are in server_access_types.go.
// permissionBody, permissionPatchBody are in server_permissions.go.
