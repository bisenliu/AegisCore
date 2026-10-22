package schema

import (
	"database/sql"
	"database/sql/driver"
	"uuid"

	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
)

var uuidColumnTypes = map[string]string{
	dialect.Postgres: "uuid",
	dialect.SQLite:   "uuid",
}

var uuidScanner = field.ValueScannerFunc[uuid.UUID, *sql.NullString]{
	V: func(value uuid.UUID) (driver.Value, error) {
		return value.String(), nil
	},
	S: func(value *sql.NullString) (uuid.UUID, error) {
		if !value.Valid {
			return uuid.Nil(), nil
		}
		return uuid.Parse(value.String)
	},
}

var nullableUUIDScanner = field.ValueScannerFunc[*uuid.UUID, *sql.NullString]{
	V: func(value *uuid.UUID) (driver.Value, error) {
		if value == nil {
			return nil, nil
		}
		return value.String(), nil
	},
	S: func(value *sql.NullString) (*uuid.UUID, error) {
		if !value.Valid {
			return nil, nil
		}
		parsed, err := uuid.Parse(value.String)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	},
}
