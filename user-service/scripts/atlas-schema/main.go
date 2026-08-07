// atlas-schema 是 Atlas external schema loader 的入口。
//
// 该程序由 migrations/atlas.hcl 间接调用，用 Ent 生成的 migrate metadata 输出 PostgreSQL DDL，
// 供 atlas migrate diff 与历史迁移目录做差异对比；它不连接真实业务数据库，也不写入迁移文件。
package main

import (
	"context"
	"log"
	"os"

	"entgo.io/ent/dialect"
	entsqlschema "entgo.io/ent/dialect/sql/schema"

	"github.com/aegiscore/user-service/internal/persistence/ent/migrate"
)

func main() {
	// Atlas schema loader 不连接真实数据库，只把 Ent migrate metadata 渲染为 PostgreSQL DDL 输出给 migration diff 流程。
	ctx := context.Background()
	tables, err := tablesWithoutForeignKeys()
	if err != nil {
		log.Fatal(err)
	}
	ddl, err := entsqlschema.DDL(ctx, entsqlschema.DDLArgs{
		Dialect: dialect.Postgres,
		// Ent DDL dump 不连接 Atlas dev database，Version 仅作为 PostgreSQL DDL 渲染的版本上下文；
		// 实际 migration diff 使用的 dev database 镜像由 migrations/atlas.hcl 的 dev_url 控制。
		Version: "15.0.0",
		Tables:  tables,
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.WriteString(ddl); err != nil {
		log.Fatal(err)
	}
}

func tablesWithoutForeignKeys() ([]*entsqlschema.Table, error) {
	// 生成迁移基线时有意移除数据库外键，只保留表、字段和索引 DDL；跨表一致性由应用层和业务流程维护。
	tables, err := entsqlschema.CopyTables(migrate.Tables)
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		table.ForeignKeys = nil
	}
	return tables, nil
}
