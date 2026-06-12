package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func applyMigrations(ctx context.Context, t *testing.T, dsn string) {
	t.Helper()
	db := openPostgres(t, dsn)
	migrationsDir := filepath.Join(userServiceRoot(t), "migrations")
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no SQL migrations found in %s", migrationsDir)
	}
	sort.Strings(files)
	for _, file := range files {
		applyMigrationFile(ctx, t, db, file)
	}
}

func applyMigrationFile(ctx context.Context, t *testing.T, db *sql.DB, file string) {
	t.Helper()
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read migration %s: %v", filepath.Base(file), err)
	}
	statements, err := splitSQLStatements(string(content))
	if err != nil {
		t.Fatalf("split migration %s: %v", filepath.Base(file), err)
	}
	for i, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply migration %s statement %d: %v", filepath.Base(file), i+1, err)
		}
	}
}

func userServiceRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		goMod := filepath.Join(dir, "go.mod")
		content, err := os.ReadFile(goMod)
		if err == nil && strings.Contains(string(content), "module github.com/aegiscore/user-service") {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatalf("locate user-service root from working directory")
		}
		dir = next
	}
}

func splitSQLStatements(sqlText string) ([]string, error) {
	var statements []string
	var current strings.Builder
	var dollarTag string
	inSingle := false
	inDouble := false

	for i := 0; i < len(sqlText); {
		if dollarTag != "" {
			if strings.HasPrefix(sqlText[i:], dollarTag) {
				current.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			current.WriteByte(sqlText[i])
			i++
			continue
		}

		if inSingle {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '\'' {
				if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				inSingle = false
			}
			i++
			continue
		}

		if inDouble {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '"' {
				if i+1 < len(sqlText) && sqlText[i+1] == '"' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				inDouble = false
			}
			i++
			continue
		}

		if strings.HasPrefix(sqlText[i:], "--") {
			for i < len(sqlText) && sqlText[i] != '\n' {
				i++
			}
			if i < len(sqlText) {
				current.WriteByte('\n')
				i++
			}
			continue
		}
		if strings.HasPrefix(sqlText[i:], "/*") {
			end := strings.Index(sqlText[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("unterminated block comment")
			}
			i += end + len("/*") + len("*/")
			current.WriteByte('\n')
			continue
		}

		switch sqlText[i] {
		case '\'':
			inSingle = true
			current.WriteByte(sqlText[i])
			i++
		case '"':
			inDouble = true
			current.WriteByte(sqlText[i])
			i++
		case '$':
			tag, ok := readDollarTag(sqlText[i:])
			if ok {
				dollarTag = tag
				current.WriteString(tag)
				i += len(tag)
				continue
			}
			current.WriteByte(sqlText[i])
			i++
		case ';':
			statement := strings.TrimSpace(current.String())
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
			i++
		default:
			current.WriteByte(sqlText[i])
			i++
		}
	}

	if dollarTag != "" {
		return nil, fmt.Errorf("unterminated dollar quote %s", dollarTag)
	}
	if inSingle {
		return nil, fmt.Errorf("unterminated single-quoted string")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double-quoted identifier")
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}

func readDollarTag(value string) (string, bool) {
	if value == "" || value[0] != '$' {
		return "", false
	}
	for i := 1; i < len(value); i++ {
		switch ch := value[i]; {
		case ch == '$':
			return value[:i+1], true
		case ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9':
			continue
		default:
			return "", false
		}
	}
	return "", false
}
