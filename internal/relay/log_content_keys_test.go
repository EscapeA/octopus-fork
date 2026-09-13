package relay

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// setupLogContentSettingTest 初始化设置缓存（内存 SQLite），供内容记录开关相关断言使用。
func setupLogContentSettingTest(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "log-content-keys.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
}

func setLogContentSetting(t *testing.T, key model.SettingKey, value string) {
	t.Helper()
	if err := setting.SetString(key, value); err != nil {
		t.Fatalf("SetString(%s=%s) failed: %v", key, value, err)
	}
}

// TestRelayLogContentEnabledForKey 验证「总开关 + API Key 白名单」语义：
// 白名单为空跟随总开关；白名单非空只放行列表内 Key，其余 Key 不再写大字段。
func TestRelayLogContentEnabledForKey(t *testing.T) {
	setupLogContentSettingTest(t)

	const (
		devKey = 12 // 高流量开发/调试 Key
		adKey  = 13 // 需要保留明细的业务 Key
	)

	setLogContentSetting(t, model.SettingKeyRelayLogContentEnabled, "true")
	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[]")

	if !relayLogContentEnabledForKey(devKey) || !relayLogContentEnabledForKey(adKey) {
		t.Fatalf("whitelist empty + master on: all keys should record content")
	}

	// 只保留业务 Key 的明细：开发 Key 只落元数据。
	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[13]")

	if relayLogContentEnabledForKey(devKey) {
		t.Errorf("key %d should not record content when whitelist is [13]", devKey)
	}
	if !relayLogContentEnabledForKey(adKey) {
		t.Errorf("key %d should record content when whitelist is [13]", adKey)
	}

	// 缓存必须随设置变更失效：改成 [12] 后放行关系应反转。
	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[12]")

	if !relayLogContentEnabledForKey(devKey) {
		t.Errorf("key %d should record content when whitelist is [12]", devKey)
	}
	if relayLogContentEnabledForKey(adKey) {
		t.Errorf("key %d should not record content when whitelist is [12]", adKey)
	}
}

// TestRelayLogContentEnabledForKeyMasterSwitch 验证总开关仍是主开关：
// 关闭时即便 Key 在白名单内也不记录；白名单不是「自动开启」。
func TestRelayLogContentEnabledForKeyMasterSwitch(t *testing.T) {
	setupLogContentSettingTest(t)

	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[13]")

	setLogContentSetting(t, model.SettingKeyRelayLogContentEnabled, "false")
	if relayLogContentEnabledForKey(13) {
		t.Errorf("master switch off should disable content logging even for whitelisted key")
	}

	setLogContentSetting(t, model.SettingKeyRelayLogContentEnabled, "true")
	if !relayLogContentEnabledForKey(13) {
		t.Errorf("master switch on + whitelisted key should record content")
	}
}

// TestRelayLogContentEnabledForKeyMalformedValue 验证设置值损坏时退化为
// 「跟随总开关」，不会误禁所有 Key 的内容记录。
func TestRelayLogContentEnabledForKeyMalformedValue(t *testing.T) {
	setupLogContentSettingTest(t)

	setLogContentSetting(t, model.SettingKeyRelayLogContentEnabled, "true")
	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[not-json")

	for _, id := range []int{0, 12, 13} {
		if !relayLogContentEnabledForKey(id) {
			t.Errorf("malformed whitelist should fall back to master switch for key %d", id)
		}
	}
}

// TestRelayLogContentAPIKeyIDsParsing 验证解析：忽略负数、去重、空值语义。
func TestRelayLogContentAPIKeyIDsParsing(t *testing.T) {
	setupLogContentSettingTest(t)

	setLogContentSetting(t, model.SettingKeyRelayLogContentAPIKeyIDs, "[13,-1,13,0]")
	ids := relayLogContentAPIKeyIDs()

	if len(ids) != 2 {
		t.Fatalf("expected 2 unique non-negative ids, got %d: %v", len(ids), ids)
	}
	if _, ok := ids[13]; !ok {
		t.Errorf("expected id 13 in parsed set")
	}
	if _, ok := ids[0]; !ok {
		t.Errorf("expected id 0 in parsed set")
	}
}
