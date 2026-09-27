package task

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

func TestShouldProbeAutoDisabledKey(t *testing.T) {
	at := time.Now()
	autoDisabled := model.ChannelKey{ID: 1, ChannelKey: "sk-x", Enabled: false, AutoDisabledAt: &at}
	if !shouldProbeAutoDisabledKey(autoDisabled) {
		t.Fatal("自动禁用的 Key（Enabled=false + 有标记）必须参与试活——曾因按 Enabled 过滤导致永不恢复")
	}
	manualEnabled := model.ChannelKey{ID: 1, ChannelKey: "sk-x", Enabled: true}
	if shouldProbeAutoDisabledKey(manualEnabled) {
		t.Fatal("手动启用（标记已清空）的 Key 不应被试活任务处理")
	}
	emptyKey := model.ChannelKey{ID: 1, ChannelKey: "  ", Enabled: false, AutoDisabledAt: &at}
	if shouldProbeAutoDisabledKey(emptyKey) {
		t.Fatal("没有 Key 明文的行无法试活")
	}
}
