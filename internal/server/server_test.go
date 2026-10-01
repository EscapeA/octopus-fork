package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/conf"
)

func TestResolveLocalStaticDirPrefersWebOutInDebug(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "true")
	t.Setenv(StaticDirEnv, "")
	setStaticDirConfig(t, "")

	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	mustWriteStaticIndex(t, filepath.Join(root, "web", "out", "index.html"))
	mustWriteStaticIndex(t, filepath.Join(root, "static", "out", "index.html"))

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected local static dir")
	}
	if filepath.Clean(got) != filepath.Clean(filepath.Join("web", "out")) {
		t.Fatalf("expected web/out, got %q", got)
	}
	if !conf.IsDebug() {
		t.Fatalf("expected debug mode from test env")
	}
}

func TestResolveLocalStaticDirFallsBackToStaticOutInDebug(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "true")
	t.Setenv(StaticDirEnv, "")
	setStaticDirConfig(t, "")

	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	mustWriteStaticIndex(t, filepath.Join(root, "static", "out", "index.html"))

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected local static dir")
	}
	if filepath.Clean(got) != filepath.Clean(filepath.Join("static", "out")) {
		t.Fatalf("expected static/out, got %q", got)
	}
}

func TestResolveLocalStaticDirDisabledOutsideDebug(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "false")
	t.Setenv(StaticDirEnv, "")
	setStaticDirConfig(t, "")

	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	mustWriteStaticIndex(t, filepath.Join(root, "web", "out", "index.html"))

	if got, ok := resolveLocalStaticDir(); ok || got != "" {
		t.Fatalf("expected no local static dir outside debug, got %q ok=%v", got, ok)
	}
}

// OCTOPUS_STATIC_DIR：显式指定目录时无需 debug 模式，且路径可为绝对路径、
// 无需落在进程工作目录下（容器场景：挂载目录 + 显式 env）。
func TestResolveLocalStaticDirFromEnvWithoutDebug(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "false")

	root := t.TempDir()
	webRoot := filepath.Join(root, "srv-web")
	mustWriteStaticIndex(t, filepath.Join(webRoot, "index.html"))

	// 进程工作目录故意切到别处，验证不依赖 CWD 探测。
	otherWD := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(otherWD); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	t.Setenv(StaticDirEnv, webRoot)

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected env static dir to be used without debug mode")
	}
	if filepath.Clean(got) != filepath.Clean(webRoot) {
		t.Fatalf("expected %q, got %q", webRoot, got)
	}
	if conf.IsDebug() {
		t.Fatalf("expected non-debug mode")
	}
}

// 显式 env 优先于 debug 模式下的 web/out 探测。
func TestResolveLocalStaticDirEnvOverridesDebugProbe(t *testing.T) {
	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	t.Setenv("OCTOPUS_DEBUG", "true")
	mustWriteStaticIndex(t, filepath.Join(root, "web", "out", "index.html"))

	envDir := filepath.Join(root, "mounted-web")
	mustWriteStaticIndex(t, filepath.Join(envDir, "index.html"))
	t.Setenv(StaticDirEnv, envDir)

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected env static dir")
	}
	if filepath.Clean(got) != filepath.Clean(envDir) {
		t.Fatalf("expected env dir %q to win over web/out, got %q", envDir, got)
	}
}

// env 指向无效目录（无 index.html）时宁可回落到内嵌资源，也不静默改用其他目录。
func TestResolveLocalStaticDirInvalidEnvDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	t.Setenv("OCTOPUS_DEBUG", "true")
	mustWriteStaticIndex(t, filepath.Join(root, "web", "out", "index.html"))

	emptyDir := filepath.Join(root, "empty-web")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv(StaticDirEnv, emptyDir)

	if got, ok := resolveLocalStaticDir(); ok || got != "" {
		t.Fatalf("expected embedded fallback when env dir is invalid, got %q ok=%v", got, ok)
	}
}

// setStaticDirConfig 设置 config.json 形式的 server.static_dir 并在用例结束后还原，
// 避免污染同包其他用例（conf.AppConfig 是进程级全局）。
func setStaticDirConfig(t *testing.T, dir string) {
	t.Helper()
	prev := conf.AppConfig.Server.StaticDir
	conf.AppConfig.Server.StaticDir = dir
	t.Cleanup(func() { conf.AppConfig.Server.StaticDir = prev })
}

// server.static_dir：config.json 形式同样无需 debug 模式。
func TestResolveLocalStaticDirFromConfigWithoutDebug(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "false")
	t.Setenv(StaticDirEnv, "")

	root := t.TempDir()
	webRoot := filepath.Join(root, "srv-web")
	mustWriteStaticIndex(t, filepath.Join(webRoot, "index.html"))

	otherWD := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(otherWD); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	setStaticDirConfig(t, webRoot)

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected config static dir to be used without debug mode")
	}
	if filepath.Clean(got) != filepath.Clean(webRoot) {
		t.Fatalf("expected %q, got %q", webRoot, got)
	}
}

// env 优先于 config.json。
func TestResolveLocalStaticDirEnvOverridesConfig(t *testing.T) {
	t.Setenv("OCTOPUS_DEBUG", "false")

	root := t.TempDir()
	envDir := filepath.Join(root, "from-env")
	cfgDir := filepath.Join(root, "from-config")
	mustWriteStaticIndex(t, filepath.Join(envDir, "index.html"))
	mustWriteStaticIndex(t, filepath.Join(cfgDir, "index.html"))

	t.Setenv(StaticDirEnv, envDir)
	setStaticDirConfig(t, cfgDir)

	got, ok := resolveLocalStaticDir()
	if !ok {
		t.Fatalf("expected a static dir")
	}
	if filepath.Clean(got) != filepath.Clean(envDir) {
		t.Fatalf("expected env dir %q to win over config dir, got %q", envDir, got)
	}
}

// server.static_dir 无效时同样不静默改用其他目录（回落到内嵌资源）。
func TestResolveLocalStaticDirInvalidConfigDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	t.Setenv("OCTOPUS_DEBUG", "true")
	t.Setenv(StaticDirEnv, "")
	mustWriteStaticIndex(t, filepath.Join(root, "web", "out", "index.html"))

	emptyDir := filepath.Join(root, "empty-web")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setStaticDirConfig(t, emptyDir)

	if got, ok := resolveLocalStaticDir(); ok || got != "" {
		t.Fatalf("expected embedded fallback when config dir is invalid, got %q ok=%v", got, ok)
	}
}

func mustWriteStaticIndex(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
