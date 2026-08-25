// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qualcomm/GenieX/cli/internal/config"
	"github.com/qualcomm/GenieX/cli/internal/render"
	"github.com/qualcomm/GenieX/cli/server/service"
)

const shutdownTimeout = 30 * time.Second

// hostBindingHint suggests --host 0.0.0.0 when bound to loopback, else "".
// Malformed hosts are treated as non-loopback — better silent than misleading.
func hostBindingHint(host string) string {
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return ""
	}
	loopback := strings.EqualFold(h, "localhost")
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	if !loopback {
		return ""
	}
	return fmt.Sprintf("Bound to loopback only. To expose on your network, restart with --host 0.0.0.0:%s", port)
}

// @Title		GenieX Server
// @Version	0.0.0
// @BasePath	/v1
func Serve() {
	service.Init()
	defer service.DeInit()

	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	RegisterRoot(engine)
	RegisterAPIv1(engine)
	RegisterSwagger(engine)

	cfg := config.Get()
	certFile := cfg.CertFile
	keyFile := cfg.KeyFile

	// Determine whether to serve over HTTPS
	if cfg.HTTPS {
		// Verify that certificate and key files exist
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			fmt.Println(render.GetTheme().Error.Sprintf("HTTPS Certificate file not found: %s", certFile))
			return
		}
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			fmt.Println(render.GetTheme().Error.Sprintf("HTTPS Key file not found: %s", keyFile))
			return
		}

		fmt.Println(render.GetTheme().Info.Sprintf("HTTPS enabled: cert=%s key=%s", certFile, keyFile))
		fmt.Println(render.GetTheme().Info.Sprintf("Local hosting on https://%s/", cfg.Host))
	} else {
		fmt.Println(render.GetTheme().Info.Sprintf("Local hosting on http://%s/", cfg.Host))
	}
	if hint := hostBindingHint(cfg.Host); hint != "" {
		fmt.Println(render.GetTheme().Info.Sprint(hint))
	}

	// Without this the runtime kills the process on Ctrl+C, skipping the
	// deferred DeInit and leaking the model's NPU buffers until reboot.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: cfg.Host, Handler: engine}
	errCh := make(chan error, 1)
	go func() {
		if cfg.HTTPS {
			errCh <- srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println(render.GetTheme().Error.Sprintf("HTTP/HTTPS Server Error: %v", err))
		}
	case <-ctx.Done():
		// Restore the default disposition so a second Ctrl+C can still abort a
		// teardown that a stuck request would otherwise hold up.
		stop()
		fmt.Println(render.GetTheme().Info.Sprint("Shutting down, releasing model resources..."))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Println(render.GetTheme().Warning.Sprintf("Server shutdown: %v", err))
		}
	}
}
