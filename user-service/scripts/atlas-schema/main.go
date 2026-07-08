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
	tables, err := entsqlschema.CopyTables(migrate.Tables)
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		table.ForeignKeys = nil
	}
	return tables, nil
}
