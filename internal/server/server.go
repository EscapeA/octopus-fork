package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	_ "github.com/lingyuins/octopus/internal/server/handlers"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/static"
)

var httpSrv http.Server

func Start() error {
	if conf.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// 默认不信任任何代理：c.ClientIP() 只返回 TCP 直连地址，防止 X-Forwarded-For
	// 伪造绕过登录限流和 API Key IP 白名单（C-01）。反代/Docker 部署需通过
	// server.trusted_proxies（或 OCTOPUS_SERVER_TRUSTED_PROXIES）配置实际代理网段，
	// 否则日志/限流/白名单看到的都是网关地址（如 Docker 的 172.17.0.1）。
	_ = setTrustedProxies(r)
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		c.Abort()
	}))

	if conf.IsDebug() {
		r.Use(middleware.Logger())
	}
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Cors())
	r.Use(middleware.AuditManagementWrite())
	if localStaticDir, ok := resolveLocalStaticDir(); ok {
		log.Infof("serving frontend static assets from local directory: %s", localStaticDir)
		r.Use(middleware.StaticLocal("/", localStaticDir))
	} else if static.StaticFS != nil {
		r.Use(middleware.StaticEmbed("/", static.StaticFS))
	} else {
		log.Warnf("frontend static assets are not embedded; API endpoints remain available, but the management UI requires building the web app first")
	}

	if err := router.RegisterAll(r); err != nil {
		return fmt.Errorf("register routes: %w", err)
	}

	httpSrv.Addr = fmt.Sprintf("%s:%d", conf.AppConfig.Server.Host, conf.AppConfig.Server.Port)
	httpSrv.Handler = r
	ln, err := net.Listen("tcp", httpSrv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", httpSrv.Addr, err)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("http server panic recovered: %v", r)
			}
		}()
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Errorf("http server listen and serve error: %v", err)
		}
	}()
	return nil
}

// setTrustedProxies 根据配置配置可信代理 CIDR 列表。
//   - 空值（默认）：不信任任何代理，c.ClientIP() 只返回 TCP 直连地址。
//   - "*"：信任所有来源（等价于 Gin 旧行为，仅开发用）。
//   - CIDR/IP 逗号分隔列表（如 "172.17.0.0/16,10.0.0.0/8"）：仅信任这些网段。
func setTrustedProxies(r *gin.Engine) error {
	raw := strings.TrimSpace(conf.AppConfig.Server.TrustedProxies)
	if raw == "" {
		return r.SetTrustedProxies(nil)
	}
	if raw == "*" {
		return r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	}
	var proxies []string
	for _, p := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(p); v != "" {
			proxies = append(proxies, v)
		}
	}
	if len(proxies) == 0 {
		return r.SetTrustedProxies(nil)
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		return fmt.Errorf("invalid trusted_proxies config %q: %w", raw, err)
	}
	log.Infof("trusted proxies configured: %v", proxies)
	return nil
}
func Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return httpSrv.Shutdown(ctx)
}

// ListenSignal waits for SIGINT/SIGTERM and then calls Close for graceful shutdown.
func ListenSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("received signal: %v, shutting down gracefully", sig)
	if err := Close(); err != nil {
		log.Errorf("shutdown error: %v", err)
	}
}

func resolveLocalStaticDir() (string, bool) {
	if !conf.IsDebug() {
		return "", false
	}

	for _, dir := range []string{"web/out", "static/out"} {
		indexPath := filepath.Join(dir, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			return dir, true
		}
	}

	return "", false
}
