package dbmigration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/backup"
)

type SaveDatabaseConfigFunc func(dbType, path string) error

var saveDatabaseConfig SaveDatabaseConfigFunc = conf.SaveDatabaseConfig

func SetSaveDatabaseConfigFuncForTest(fn SaveDatabaseConfigFunc) func() {
	old := saveDatabaseConfig
	saveDatabaseConfig = fn
	return func() { saveDatabaseConfig = old }
}

func ValidateRequest(req model.DatabaseMigrationRequest) (model.DatabaseMigrationRequest, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.Path = strings.TrimSpace(req.Path)
	if req.Type == "postgresql" {
		req.Type = "postgres"
	}
	if req.Type != "sqlite" && req.Type != "mysql" && req.Type != "postgres" {
		return req, fmt.Errorf("unsupported database type: %s", req.Type)
	}
	if req.Path == "" {
		return req, fmt.Errorf("database path is required")
	}
	// For SQLite, constrain the path to within the data directory to prevent
	// arbitrary file read/write via path traversal (2C-03). MySQL/Postgres
	// use network addresses, not filesystem paths, so they are not constrained.
	if req.Type == "sqlite" {
		absPath, err := filepath.Abs(req.Path)
		if err != nil {
			return req, fmt.Errorf("resolve db path: %w", err)
		}
		dataDir, err := filepath.Abs(conf.DataDir())
		if err != nil {
			return req, fmt.Errorf("resolve data dir: %w", err)
		}
		rel, err := filepath.Rel(dataDir, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			return req, fmt.Errorf("sqlite path must be within the data directory (%s)", dataDir)
		}
	}
	return req, nil
}

func TestConnection(ctx context.Context, req model.DatabaseMigrationRequest) error {
	req, err := ValidateRequest(req)
	if err != nil {
		return err
	}
	target, err := db.OpenStandalone(req.Type, req.Path, false)
	if err != nil {
		return err
	}
	sqlDB, err := target.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.PingContext(ctx)
}

func Migrate(ctx context.Context, req model.DatabaseMigrationRequest) (*model.DatabaseMigrationResult, error) {
	req, err := ValidateRequest(req)
	if err != nil {
		return nil, err
	}
	dump, err := backup.ExportAll(ctx, req.IncludeLogs, req.IncludeStats)
	if err != nil {
		return nil, fmt.Errorf("export current database: %w", err)
	}

	target, err := db.OpenStandalone(req.Type, req.Path, false)
	if err != nil {
		return nil, fmt.Errorf("open target database: %w", err)
	}
	sqlDB, err := target.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	if err := db.Migrate(target); err != nil {
		return nil, fmt.Errorf("migrate target schema: %w", err)
	}
	importResult, err := backup.ImportWithModeToDB(ctx, target, dump, model.ImportModeFull)
	if err != nil {
		return nil, fmt.Errorf("import target database: %w", err)
	}
	if err := saveDatabaseConfig(req.Type, req.Path); err != nil {
		return nil, err
	}

	// 源库为 SQLite、目标库为非 SQLite 时，关闭旧 SQLite 连接并删除文件，
	// 避免进程持续读盘陷入 D 状态（issue #118）。数据已导入目标库、新配置已
	// 写入 config.json，旧 SQLite 不再需要。调用后进程处于「必须重启」终态。
	var cleanedFiles []string
	if db.IsSQLite() && req.Type != "sqlite" {
		cleanedFiles = db.CloseAndCleanupSQLite()
	}

	return &model.DatabaseMigrationResult{
		Type:          req.Type,
		Path:          req.Path,
		IncludeLogs:   req.IncludeLogs,
		IncludeStats:  req.IncludeStats,
		RestartNeeded: true,
		CleanedFiles:  cleanedFiles,
		ImportResult:  *importResult,
	}, nil
}
