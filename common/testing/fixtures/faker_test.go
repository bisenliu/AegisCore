package fixtures

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFakerGeneratesReadableUniqueValues(t *testing.T) {
	faker := NewFaker(t)

	first := faker.UniqueSuffix()
	second := faker.UniqueSuffix()
	require.NotEmpty(t, first)
	require.NotEmpty(t, second)
	require.NotEqual(t, first, second)

	username := faker.Username("Admin User")
	require.True(t, strings.HasPrefix(username, "admin-user-"))

	email := faker.Email("Login")
	require.True(t, strings.HasPrefix(email, "login-"))
	require.True(t, strings.HasSuffix(email, "@example.test"))

	name := faker.Name("Display Name")
	require.True(t, strings.HasPrefix(name, "Display Name "))

	_, err := uuid.Parse(faker.UUIDString())
	require.NoError(t, err)
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
		require.NotContains(t, seen, value)
		seen[value] = struct{}{}
	}
}
