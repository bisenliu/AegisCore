//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const entSchemaURL = "ent://ent/schema"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var cmd *exec.Cmd
	switch os.Args[1] {
	case "inspect":
		cmd = exec.Command("atlas", "schema", "inspect", "--url", entSchemaURL, "--dev-url", devURL(), "--format", "{{ sql . }}")
	case "diff":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "migration name is required")
			usage()
			os.Exit(2)
		}
		cmd = exec.Command("atlas", "migrate", "diff", os.Args[2], "--env", "local")
		cmd.Env = append(os.Environ(), "GOWORK=off")
	default:
		usage()
		os.Exit(2)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cmd.Env == nil {
		cmd.Env = append(os.Environ(), "GOWORK=off")
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "migration command failed: %v\n", err)
		os.Exit(1)
	}
}

func devURL() string {
	if url := os.Getenv("ATLAS_DEV_URL"); url != "" {
		return url
	}
	return "docker://postgres/15/dev?search_path=public"
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./ent/migrate/main.go inspect")
	fmt.Fprintln(os.Stderr, "  go run ./ent/migrate/main.go diff <migration-name>")
}
