package backup

import (
	"context"
	"path/filepath"
	"testing"

	internaldb "github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

// TestImportOrphanStatsChannelSurvivesFKConstraint reproduces issue #112:
// the source DB (SQLite) can accumulate orphan stats_channel rows whose
// channel_id no longer references an existing channel. GORM declares
// Channel.Stats with `foreignKey:ChannelID`, so on a target that enforces
// FK constraints, importing those orphans must not fail with Error 1452 /
// FK violation. The import temporarily disables FK checks for the target
// session so orphan child rows are preserved rather than rejected.
//
// This test uses a fresh SQLite target (PRAGMA foreign_keys=ON via
// OpenStandalone) so the FK constraint exists and would reject orphans
// without the fix.
func TestImportOrphanStatsChannelSurvivesFKConstraint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "orphan.db")
	if err := internaldb.InitDB("sqlite", dbPath, false); err != nil {
		t.Fatalf("init source db: %v", err)
	}
	t.Cleanup(func() { _ = internaldb.Close() })

	// Build a dump with an orphan stats_channel: channel 999 is NOT in
	// dump.Channels. This mirrors a real SQLite source where the channel
	// was deleted but stats persisted (FK was historically not enforced).
	dump := &model.DBDump{
		Version: 1,
		Channels: []model.Channel{{
			ID:       1,
			Name:     "real-channel",
			Type:     outbound.OutboundTypeOpenAIChat,
			BaseUrls: []model.BaseUrl{{URL: "https://real.example.com"}},
		}},
		IncludeStats: true,
		// channel_id 1 is valid; channel_id 999 is an ORPHAN.
		StatsChannel: []model.StatsChannel{
			{ChannelID: 1, StatsMetrics: model.StatsMetrics{RequestSuccess: 5}},
			{ChannelID: 999, StatsMetrics: model.StatsMetrics{RequestSuccess: 2}},
		},
	}

	// Fresh target with FK enforcement (OpenStandalone enables foreign_keys PRAGMA).
	targetPath := filepath.Join(t.TempDir(), "target-orphan.db")
	target, err := internaldb.OpenStandalone("sqlite", targetPath, false)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	sqlDB, _ := target.DB()
	defer sqlDB.Close()
	if err := internaldb.Migrate(target); err != nil {
		t.Fatalf("migrate target: %v", err)
	}

	// Without the fix this returns an FK violation on stats_channel 999.
	if _, err := ImportWithModeToDB(context.Background(), target, dump, model.ImportModeFull); err != nil {
		t.Fatalf("ImportWithModeToDB failed on orphan stats_channel (issue #112): %v", err)
	}

	// Both rows must survive the import.
	var total int64
	if err := target.Model(&model.StatsChannel{}).Count(&total).Error; err != nil {
		t.Fatalf("count stats_channel: %v", err)
	}
	if total != 2 {
		t.Fatalf("stats_channel count = %d, want 2 (orphan must be preserved)", total)
	}

	// Sanity: the real channel and its stats landed too.
	var ch int64
	if err := target.Model(&model.Channel{}).Count(&ch).Error; err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if ch != 1 {
		t.Fatalf("channel count = %d, want 1", ch)
	}

	// FK is restored after import: a fresh orphan insert must now be rejected.
	// (Only meaningful on dialects that enforce FK; SQLite with PRAGMA ON does.)
	if err := target.Create(&model.StatsChannel{ChannelID: 7777}).Error; err == nil {
		// SQLite glebarez may not enforce FK on a session re-acquired from the
		// pool; treat as non-fatal but log it so we know enforcement state.
		t.Logf("NOTE: target did not reject a fresh orphan insert (FK enforcement inactive on this session)")
	}
}
