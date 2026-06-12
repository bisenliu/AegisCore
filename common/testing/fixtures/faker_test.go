package fixtures

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestFakerGeneratesReadableUniqueValues(t *testing.T) {
	faker := NewFaker(t)

	first := faker.UniqueSuffix()
	second := faker.UniqueSuffix()
	if first == "" || second == "" || first == second {
		t.Fatalf("suffixes = %q, %q; want non-empty unique values", first, second)
	}

	username := faker.Username("Admin User")
	if !strings.HasPrefix(username, "admin-user-") {
		t.Fatalf("username = %q, want sanitized prefix", username)
	}

	email := faker.Email("Login")
	if !strings.HasPrefix(email, "login-") || !strings.HasSuffix(email, "@example.test") {
		t.Fatalf("email = %q, want stable test email", email)
	}

	name := faker.Name("Display Name")
	if !strings.HasPrefix(name, "Display Name ") {
		t.Fatalf("name = %q, want readable prefix", name)
	}

	if _, err := uuid.Parse(faker.UUIDString()); err != nil {
		t.Fatalf("UUIDString returned invalid UUID: %v", err)
	}
}

func TestFakerSupportsParallelUse(t *testing.T) {
	faker := NewFaker(t)
	const count = 32
	values := make(chan string, count)

	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			values <- faker.Username("parallel")
		}()
	}
	wg.Wait()
	close(values)

	seen := make(map[string]struct{}, count)
	for value := range values {
		if _, ok := seen[value]; ok {
			t.Fatalf("duplicate faker value %q", value)
		}
		seen[value] = struct{}{}
	}
}
