package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
)

// newTrustedProxyEngine 构造一个 gin.Engine 并应用 setTrustedProxies，
// 模拟反向代理转发请求：RemoteAddr 来自"代理"（172.17.0.1），
// X-Forwarded-For 携带真实客户端 IP。
func newTrustedProxyEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := setTrustedProxies(r); err != nil {
		t.Fatalf("setTrustedProxies: %v", err)
	}
	r.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	return r
}

func doProxyRequest(t *testing.T, r *gin.Engine, xff string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	// 172.17.0.1 = Docker bridge 网关，模拟反代容器直连 Octopus
	req.RemoteAddr = "172.17.0.1:12345"
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

// 默认（空配置）：不信任任何代理，返回 TCP 直连地址（网关），忽略 XFF。
func TestSetTrustedProxies_DefaultReturnsRemoteAddr(t *testing.T) {
	conf.AppConfig.Server.TrustedProxies = ""
	r := newTrustedProxyEngine(t)

	got := doProxyRequest(t, r, "1.2.3.4")
	if got != "172.17.0.1" {
		t.Fatalf("default: expected remote addr 172.17.0.1, got %q", got)
	}
}

// 配置代理网段：信任 172.17.0.0/16，读取 XFF 得到真实客户端 IP。
func TestSetTrustedProxies_CidrTrustsXForwardedFor(t *testing.T) {
	conf.AppConfig.Server.TrustedProxies = "172.17.0.0/16"
	r := newTrustedProxyEngine(t)

	got := doProxyRequest(t, r, "1.165.0.250")
	if got != "1.165.0.250" {
		t.Fatalf("cidr: expected XFF 1.165.0.250, got %q", got)
	}
}

// "*" 信任所有来源（等价 Gin 旧行为），读取 XFF。
func TestSetTrustedProxies_WildcardTrustsAll(t *testing.T) {
	conf.AppConfig.Server.TrustedProxies = "*"
	r := newTrustedProxyEngine(t)

	got := doProxyRequest(t, r, "8.8.8.8")
	if got != "8.8.8.8" {
		t.Fatalf("wildcard: expected XFF 8.8.8.8, got %q", got)
	}
}

// 代理网段不在信任列表内：忽略 XFF，返回直连地址。
func TestSetTrustedProxies_UntrustedProxyIgnoresXFF(t *testing.T) {
	conf.AppConfig.Server.TrustedProxies = "10.0.0.0/8"
	r := newTrustedProxyEngine(t)

	// 172.17.0.1 不在 10.0.0.0/8 信任范围，XFF 应被忽略
	got := doProxyRequest(t, r, "1.2.3.4")
	if got != "172.17.0.1" {
		t.Fatalf("untrusted proxy: expected 172.17.0.1, got %q", got)
	}
}
