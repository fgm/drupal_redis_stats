package dock

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest"
)

const testUser = "postgres"
const testPassword = "password"
const testHost = "localhost"
const testDbName = "phone_numbers"

var (
	pool     *dockertest.Pool
	testPort string
)

// getAdapter retrieves the Postgres adapter with test credentials
func getAdapter() (*PgAdapter, error) {
	return NewAdapter(testHost, testPort, testUser, testDbName, WithPassword(testPassword))
}

// setup instantiates a Postgres docker container and attempts to connect to it via a new adapter
func setup() *dockertest.Resource {
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not connect to docker: %s", err)
	}

	// Pulls an image, creates a container based on it and runs it
	resource, err := pool.Run("postgres", "14", []string{fmt.Sprintf("POSTGRES_PASSWORD=%s", testPassword), fmt.Sprintf("POSTGRES_DB=%s", testDbName)})
	if err != nil {
		log.Fatalf("could not start resource: %s", err)
	}
	testPort = resource.GetPort("5432/tcp") // Set port used to communicate with Postgres

	var adapter *PgAdapter
	// Exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		adapter, err = getAdapter()
		return err
	}); err != nil {
		log.Fatalf("could not connect to docker: %s", err)
	}

	initTestAdapter(adapter)

	return resource
}

func clear(res *dockertest.Resource) {
	if res != nil {
		if err := res.Close(); err != nil {
			log.Printf("failed closing resource: %v", err)
		}
	}
}

func TestMain(m *testing.M) {
	r := setup()
	code := m.Run()
	clear(r)
	os.Exit(code)
}

func TestCreatePhoneNumber(t *testing.T) {
	testNumber := "1234566656"
	adapter, err := getAdapter()
	if err != nil {
		t.Fatalf("error creating new test adapter: %v", err)
	}

	cases := []struct {
		error       bool
		description string
	}{
		{
			description: "Should succeed with valid creation of a phone number",
		},
		{
			description: "Should fail if database connection closed",
			error:       true,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			if c.error {
				adapter.conn.Close()
			}
			id, err := adapter.CreatePhoneNumber(testNumber)
			if !c.error && err != nil {
				t.Errorf("expecting no error but received: %v", err)
			} else if !c.error { // Remove test number from db so not captured by following tests
				err = adapter.RemovePhoneNumber(id)
				if err != nil {
					t.Fatalf("error removing test number from database")
				}
			}
		})
	}
}
