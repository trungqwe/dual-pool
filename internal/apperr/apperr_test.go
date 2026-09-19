package apperr

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestExitCategoryValues(t *testing.T) {
	tests := map[Category]int{
		CategorySuccess:       0,
		CategoryUsage:         2,
		CategoryCompatibility: 10,
		CategoryConfig:        20,
		CategoryAuth:          30,
		CategoryService:       40,
		CategorySecurity:      50,
		CategoryTest:          60,
	}
	for category, want := range tests {
		if got := category.ExitCode(); got != want {
			t.Errorf("%s exit = %d, want %d", category, got, want)
		}
	}
}

func TestRegistryCompletenessAndMappings(t *testing.T) {
	expected := map[Code]Category{
		"PORT_IN_USE":                 CategoryService,
		"PROCESS_IDENTITY_MISMATCH":   CategorySecurity,
		"UPSTREAM_VERSION_MISMATCH":   CategoryCompatibility,
		"MANAGEMENT_UNAVAILABLE":      CategoryService,
		"OAUTH_TIMEOUT":               CategoryAuth,
		"ACCOUNT_INELIGIBLE":          CategoryAuth,
		"CONFIG_CONFLICT":             CategoryConfig,
		"AG_SCHEMA_UNSUPPORTED":       CategoryCompatibility,
		"AG_POOL_UNAVAILABLE":         CategoryService,
		"CODEX_PICKER_ROUTE_MISMATCH": CategoryCompatibility,
		"CATALOG_VERSION_MISMATCH":    CategoryCompatibility,
		"POOL_ISOLATION_VIOLATION":    CategorySecurity,
		"ROLLBACK_CONFLICT":           CategoryConfig,
		"SECRET_LEAK_DETECTED":        CategorySecurity,
		"INVALID_COMMAND":             CategoryUsage,
		"INVALID_ARGUMENT":            CategoryUsage,
		"DATA_ROOT_UNAVAILABLE":       CategoryConfig,
	}
	registry := Registry()
	if len(registry) != len(expected) {
		t.Fatalf("registry has %d entries, want %d", len(registry), len(expected))
	}
	for code, category := range expected {
		definition, ok := registry[code]
		if !ok {
			t.Errorf("missing code %s", code)
			continue
		}
		if definition.Category != category || definition.Message == "" {
			t.Errorf("invalid definition for %s: %#v", code, definition)
		}
	}
	if !reflect.DeepEqual(registry, Registry()) {
		t.Fatal("registry is not deterministic")
	}
}

func TestWrappedCauseNeverLeaks(t *testing.T) {
	sensitive := `open C:\Users\Example\secret: Bearer SENTINEL_SECRET_123456789 user@example.invalid`
	cause := errors.New(sensitive)
	err, constructionErr := New(CodeConfigConflict, cause)
	if constructionErr != nil {
		t.Fatal(constructionErr)
	}
	if !errors.Is(err, cause) {
		t.Fatal("cause is not internally inspectable")
	}
	human := err.Error()
	payload, marshalErr := json.Marshal(err.Envelope())
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, output := range []string{human, string(payload)} {
		for _, forbidden := range []string{sensitive, `C:\Users\Example`, "SENTINEL_SECRET", "user@example.invalid", "cause", "stack"} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("safe output leaked %q in %q", forbidden, output)
			}
		}
	}
	if human != "CONFIG_CONFLICT: configuration changed since it was inspected" {
		t.Fatalf("unexpected human rendering: %q", human)
	}
	if got := err.Envelope().ExitCode; got != 20 {
		t.Fatalf("envelope exit = %d, want 20", got)
	}
}

func TestUnknownCodeCannotBecomeUsageError(t *testing.T) {
	cause := errors.New("Bearer SENTINEL_SECRET_123456789")
	err, constructionErr := New(Code("UNREGISTERED_PRIVATE_VALUE"), cause)
	if err != nil || constructionErr == nil {
		t.Fatalf("unknown code returned err=%v constructionErr=%v", err, constructionErr)
	}
	if strings.Contains(constructionErr.Error(), "UNREGISTERED_PRIVATE_VALUE") || strings.Contains(constructionErr.Error(), "SENTINEL_SECRET") || strings.Contains(constructionErr.Error(), "INVALID_ARGUMENT") {
		t.Fatal("construction error leaked input or became a usage error")
	}
}
