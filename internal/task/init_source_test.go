package task

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// loadInitSource 读取 internal/task/init.go 源码，并归一化行尾，供各处的
// 「源码守卫」测试断言任务注册写法。
//
// 为什么用源码守卫而不是跑 Init()：Register 的任务体是注册时捕获的闭包，且
// tasks 是包级全局注册表（重复注册同名任务会被 "already registered, skipping"
// 静默跳过）。要在运行时取出某个任务闭包做断言必须触发真实 Init()，那会连带
// 启动 serial writer / flush worker / 注册十余个任务，污染全局注册表且无法复原，
// 部分任务还会真实出站。源码守卫精确、零副作用、不受注册表状态影响。
//
// 归一化是必需的：仓库 core.autocrlf=true，工作区里的 .go 文件实际是 CRLF。
// 若不先把 \r\n 换成 \n，按 "\n\t\t})" 切闭包体会直接切不到。
//
// 注：上游原有同名 helper（随价格更新任务的超时守卫一起引入），本 fork 已随
// 「计费人民币化」移除价格更新任务与对应守卫，这里保留 helper 供其余源码守卫复用。
func loadInitSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src, err := os.ReadFile(filepath.Clean(filepath.Join(filepath.Dir(file), "init.go")))
	if err != nil {
		t.Fatalf("read init.go: %v", err)
	}
	return strings.ReplaceAll(string(src), "\r\n", "\n")
}
