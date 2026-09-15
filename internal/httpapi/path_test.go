package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/DejavuMoe/uPaste/internal/capability"
)

func TestShareIDAttackMatrix(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("PLAIN", "path attack", nil)
	valid := created.Share.ID

	malformed := []string{
		"",
		"short",
		strings.Repeat("A", 21),
		strings.Repeat("A", 23),
		valid + "=",
		valid[:len(valid)-1] + "+",
		valid[:len(valid)-1] + "/",
		valid[:len(valid)-1] + "!",
		valid + "/extra",
		"/" + valid,
		"./" + valid,
		"..",
		"%2e%2e",
		strings.Repeat("A", 4096),
	}
	for _, id := range malformed {
		t.Run("api:"+id, func(t *testing.T) {
			response := env.request(http.MethodGet, "/api/v1/shares/"+id, nil)
			if response.Code != http.StatusNotFound {
				t.Fatalf("API malformed ID %q status = %d, want 404", id, response.Code)
			}
			if strings.Contains(response.Body.String(), "root") {
				t.Fatal("malformed API path returned frontend content")
			}
		})
		t.Run("raw:"+id, func(t *testing.T) {
			response := env.request(http.MethodGet, "/raw/"+id, nil)
			if response.Code != http.StatusNotFound {
				t.Fatalf("raw malformed ID %q status = %d, want 404", id, response.Code)
			}
			assertRawHeaders(t, response)
		})
	}

	// Query strings are ignored and do not create a second identity.
	response := env.request(http.MethodGet, "/api/v1/shares/"+valid+"?owner_token=ignored", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("valid ID with query status = %d, want 200", response.Code)
	}

	// A generated but unregistered ID must be a generic not-found.
	id, err := capability.GenerateShareID()
	if err != nil {
		t.Fatal(err)
	}
	assertError(t, env.request(http.MethodGet, "/api/v1/shares/"+id.String(), nil), http.StatusNotFound, "not_found")
}
