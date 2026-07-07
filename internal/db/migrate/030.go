package migrate

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 30,
		Up:      migrateGroupEndpointNameUniqueIndex,
	})
}

// 030: 路由分组按 endpoint_type + name 唯一，而不是按 name 全局唯一。
// 同名模型可以同时存在于不同 API 分类（例如 chat 与 embeddings），解析时也按
// endpoint_type + name 查找；旧的 groups.name 全局唯一索引会阻止这种合法分组。
func migrateGroupEndpointNameUniqueIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Group{}) {
		return nil
	}

	if err := dropGroupsNameOnlyUniqueIndexes(db); err != nil {
		return err
	}
	if !db.Migrator().HasIndex(&model.Group{}, "idx_groups_endpoint_name") {
		if err := db.Migrator().CreateIndex(&model.Group{}, "idx_groups_endpoint_name"); err != nil {
			return fmt.Errorf("create groups(endpoint_type,name) unique index: %w", err)
		}
	}
	return nil
}

func dropGroupsNameOnlyUniqueIndexes(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite":
		return dropSQLiteGroupsNameOnlyUniqueIndexes(db)
	case "mysql":
		return dropMySQLGroupsNameOnlyUniqueIndexes(db)
	case "postgres", "postgresql":
		return dropPostgresGroupsNameOnlyUniqueIndexes(db)
	default:
		for _, name := range []string{"idx_groups_name", "uni_groups_name", "name"} {
			if db.Migrator().HasIndex(&model.Group{}, name) {
				if err := db.Migrator().DropIndex(&model.Group{}, name); err != nil {
					return fmt.Errorf("drop groups.%s unique index: %w", name, err)
				}
			}
		}
		return nil
	}
}

func dropSQLiteGroupsNameOnlyUniqueIndexes(db *gorm.DB) error {
	type indexRow struct {
		Name   string
		Unique int
		Origin string
	}
	var indexes []indexRow
	if err := db.Raw(`PRAGMA index_list('groups')`).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("list sqlite groups indexes: %w", err)
	}

	nameOnlyUnique := make(map[string]struct{})
	needsRebuild := false
	for _, idx := range indexes {
		if idx.Unique != 1 {
			continue
		}
		var columns []struct{ Name string }
		if err := db.Raw(fmt.Sprintf("PRAGMA index_info(%q)", idx.Name)).Scan(&columns).Error; err != nil {
			return fmt.Errorf("inspect sqlite groups index %s: %w", idx.Name, err)
		}
		if len(columns) == 1 && columns[0].Name == "name" {
			nameOnlyUnique[idx.Name] = struct{}{}
			if strings.HasPrefix(idx.Name, "sqlite_autoindex_") || idx.Origin == "u" {
				needsRebuild = true
			}
		}
	}

	if len(nameOnlyUnique) == 0 {
		return nil
	}
	if needsRebuild {
		return rebuildSQLiteGroupsWithoutNameUnique(db, nameOnlyUnique)
	}
	for idxName := range nameOnlyUnique {
		if err := db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %q", idxName)).Error; err != nil {
			return fmt.Errorf("drop sqlite groups index %s: %w", idxName, err)
		}
	}
	return nil
}

type sqliteColumnInfo struct {
	Name      string
	Type      string
	NotNull   int
	DfltValue sql.NullString `gorm:"column:dflt_value"`
	PK        int            `gorm:"column:pk"`
}

func rebuildSQLiteGroupsWithoutNameUnique(db *gorm.DB, nameOnlyUnique map[string]struct{}) error {
	type indexDef struct {
		Name string
		SQL  sql.NullString `gorm:"column:sql"`
	}
	var indexes []indexDef
	if err := db.Raw(`SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'groups' AND sql IS NOT NULL`).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("list sqlite groups index SQL: %w", err)
	}

	var foreignKeysEnabled int
	if err := db.Raw(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled).Error; err != nil {
		return fmt.Errorf("read sqlite foreign_keys pragma: %w", err)
	}
	if err := db.Exec(`PRAGMA foreign_keys=OFF`).Error; err != nil {
		return fmt.Errorf("disable sqlite foreign_keys: %w", err)
	}
	defer func() { _ = db.Exec(fmt.Sprintf("PRAGMA foreign_keys=%d", foreignKeysEnabled)).Error }()

	return db.Transaction(func(tx *gorm.DB) error {
		var columns []sqliteColumnInfo
		if err := tx.Raw(`PRAGMA table_info('groups')`).Scan(&columns).Error; err != nil {
			return fmt.Errorf("inspect sqlite groups columns: %w", err)
		}
		if len(columns) == 0 {
			return fmt.Errorf("sqlite groups table has no columns")
		}

		columnDefs := make([]string, 0, len(columns))
		columnNames := make([]string, 0, len(columns))
		for _, column := range columns {
			columnDefs = append(columnDefs, sqliteColumnDefinition(column))
			columnNames = append(columnNames, quoteSQLiteIdent(column.Name))
		}

		tmpTable := "groups_without_name_unique"
		if err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteSQLiteIdent(tmpTable))).Error; err != nil {
			return fmt.Errorf("drop temporary sqlite groups table: %w", err)
		}
		if err := tx.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", quoteSQLiteIdent(tmpTable), strings.Join(columnDefs, ", "))).Error; err != nil {
			return fmt.Errorf("create rebuilt sqlite groups table: %w", err)
		}
		joinedColumns := strings.Join(columnNames, ", ")
		if err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", quoteSQLiteIdent(tmpTable), joinedColumns, joinedColumns, quoteSQLiteIdent("groups"))).Error; err != nil {
			return fmt.Errorf("copy sqlite groups rows: %w", err)
		}
		if err := tx.Exec(fmt.Sprintf("DROP TABLE %s", quoteSQLiteIdent("groups"))).Error; err != nil {
			return fmt.Errorf("drop old sqlite groups table: %w", err)
		}
		if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteSQLiteIdent(tmpTable), quoteSQLiteIdent("groups"))).Error; err != nil {
			return fmt.Errorf("rename rebuilt sqlite groups table: %w", err)
		}
		for _, idx := range indexes {
			if _, remove := nameOnlyUnique[idx.Name]; remove || !idx.SQL.Valid {
				continue
			}
			if err := tx.Exec(idx.SQL.String).Error; err != nil {
				return fmt.Errorf("recreate sqlite groups index %s: %w", idx.Name, err)
			}
		}
		return nil
	})
}

func sqliteColumnDefinition(column sqliteColumnInfo) string {
	columnType := strings.TrimSpace(column.Type)
	if columnType == "" {
		columnType = "TEXT"
	}
	definition := quoteSQLiteIdent(column.Name) + " " + columnType
	if column.PK > 0 {
		definition += " PRIMARY KEY"
		if strings.EqualFold(column.Name, "id") && strings.EqualFold(columnType, "INTEGER") {
			definition += " AUTOINCREMENT"
		}
	} else if column.NotNull == 1 {
		definition += " NOT NULL"
	}
	if column.DfltValue.Valid {
		definition += " DEFAULT " + column.DfltValue.String
	}
	return definition
}

func quoteSQLiteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func dropMySQLGroupsNameOnlyUniqueIndexes(db *gorm.DB) error {
	type indexRow struct {
		IndexName   string
		ColumnCount int
		NameCount   int
	}
	var indexes []indexRow
	if err := db.Raw(`
		SELECT INDEX_NAME AS index_name,
		       COUNT(*) AS column_count,
		       SUM(CASE WHEN COLUMN_NAME = 'name' THEN 1 ELSE 0 END) AS name_count
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		  AND table_name = 'groups'
		  AND non_unique = 0
		GROUP BY INDEX_NAME
	`).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("list mysql groups unique indexes: %w", err)
	}
	for _, idx := range indexes {
		if idx.ColumnCount == 1 && idx.NameCount == 1 {
			if err := db.Exec(fmt.Sprintf("ALTER TABLE `groups` DROP INDEX `%s`", idx.IndexName)).Error; err != nil {
				return fmt.Errorf("drop mysql groups index %s: %w", idx.IndexName, err)
			}
		}
	}
	return nil
}

func dropPostgresGroupsNameOnlyUniqueIndexes(db *gorm.DB) error {
	// GORM unique indexes on a single column are usually indexes, but older/manual
	// schemas may have a table constraint. Try common constraint names first.
	for _, constraint := range []string{"uni_groups_name", "idx_groups_name", "groups_name_key"} {
		_ = db.Exec(fmt.Sprintf(`ALTER TABLE "groups" DROP CONSTRAINT IF EXISTS "%s"`, constraint)).Error
	}

	type indexRow struct{ IndexName string }
	var indexes []indexRow
	if err := db.Raw(`
		SELECT i.relname AS index_name
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = 'groups'
		  AND ix.indisunique
		GROUP BY i.relname, ix.indkey
		HAVING COUNT(*) = 1 AND BOOL_AND(a.attname = 'name')
	`).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("list postgres groups unique indexes: %w", err)
	}
	for _, idx := range indexes {
		if err := db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, idx.IndexName)).Error; err != nil {
			return fmt.Errorf("drop postgres groups index %s: %w", idx.IndexName, err)
		}
	}
	return nil
}
