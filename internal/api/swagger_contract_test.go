package api_test

import (
	"strings"
	"testing"

	"github.com/ybordag/cambium/docs"
)

func TestSessionContextSwaggerUsesTextFirstContract(t *testing.T) {
	swagger := docs.SwaggerInfo.SwaggerTemplate

	for _, field := range []string{"time_text", "energy_text", "focus_text", "focus_context"} {
		if !strings.Contains(swagger, `"`+field+`"`) {
			t.Fatalf("swagger is missing session context field %q", field)
		}
	}
	for _, staleField := range []string{
		"available_minutes",
		"energy_level",
		"focus_project_id",
		"focus_label",
		"preferred_location_type",
		"open_to_outdoor_work",
		"wants_quick_wins",
	} {
		if strings.Contains(swagger, `"`+staleField+`"`) {
			t.Fatalf("swagger still advertises stale session context field %q", staleField)
		}
	}
	if !strings.Contains(swagger, `"api.SessionContextObjectRef"`) {
		t.Fatal("swagger is missing the focus_context object-ref schema")
	}
}
