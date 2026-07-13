package main

import (
	"context"
	"log"
	"os"

	"entgo.io/ent/dialect"
	entsqlschema "entgo.io/ent/dialect/sql/schema"

	"github.com/aegiscore/user-service/ent/migrate"
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
