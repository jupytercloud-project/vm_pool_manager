package grpc

import (
	"control_center/config"
	"control_center/models"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func decodePathSegment(s string) string {
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}

// handleJupyterProxy proxies all requests (HTTP + WebSocket) to a JupyterLab VM.
// URL format: /api/jupyter-proxy/{pool_id}/{user_id}/{...rest}
//
// This solves the mixed-content problem: the browser uses HTTPS (via Caddy),
// which forwards to the control center, which forwards HTTP to the private VM.
func handleJupyterProxy(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/jupyter-proxy/"), "/", 3)
	if len(parts) < 2 {
		http.Error(w, "usage: /api/jupyter-proxy/{pool_id}/{user_id}/...", http.StatusBadRequest)
		return
	}
	poolID := decodePathSegment(parts[0])
	userID := decodePathSegment(parts[1])

	var server models.Server
	if err := config.Database.
		Where("serverpool_id = ? AND user_id = ?", poolID, userID).
		First(&server).Error; err != nil {
		http.Error(w, "VM not found for pool "+poolID, http.StatusNotFound)
		return
	}
	if server.IP_Address == "" {
		http.Error(w, "VM has no IP address yet", http.StatusServiceUnavailable)
		return
	}

	var pool models.Serverpool
	port := 8888
	if err := config.Database.Where("serverpool_id = ? AND user_id = ?", poolID, userID).First(&pool).Error; err == nil && pool.AppPort > 0 {
		port = pool.AppPort
	}

	targetBase, _ := url.Parse(fmt.Sprintf("http://%s:%d", server.IP_Address, port))

	proxy := httputil.NewSingleHostReverseProxy(targetBase)
	// Remove headers that prevent iframe embedding
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[jupyter-proxy] %s/%s → %s error: %v", poolID, userID, targetBase.Host, err)
		http.Error(w, "JupyterLab unreachable: "+err.Error(), http.StatusBadGateway)
	}

	// Rewrite path: do not strip the prefix so it matches JupyterLab's base_url
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = targetBase.Scheme
	r2.URL.Host = targetBase.Host
	r2.Host = targetBase.Host

	// JupyterLab checks Origin — set it to match the target so it accepts the request
	r2.Header.Set("Origin", targetBase.String())
	r2.Header.Set("X-Forwarded-Proto", "http")

	proxy.ServeHTTP(w, r2)
}
