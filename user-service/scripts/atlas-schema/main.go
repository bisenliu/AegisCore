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
	ddl, err := entsqlschema.DDL(ctx, entsqlschema.DDLArgs{
		Dialect: dialect.Postgres,
		Version: "15.0.0",
		Tables:  tablesWithoutForeignKeys(),
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.WriteString(ddl); err != nil {
		log.Fatal(err)
	}
}

func tablesWithoutForeignKeys() []*entsqlschema.Table {
	tables := make([]*entsqlschema.Table, 0, len(migrate.Tables))
	for _, table := range migrate.Tables {
		clone := *table
		clone.ForeignKeys = nil
		tables = append(tables, &clone)
	}
	return tables
}
