package runtimeupdate_test

import (
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/instance"
	"github.com/trungqwe/dual-pool/internal/runtimeupdate"
	"github.com/trungqwe/dual-pool/internal/state"
)

func TestRuntimePublicAPIHasNoStateOrLifecycleAuthority(t *testing.T) {
	typeOfRuntime := reflect.TypeOf(runtimeupdate.Runtime{})
	exported := map[string]reflect.Type{}
	for i := 0; i < typeOfRuntime.NumField(); i++ {
		field := typeOfRuntime.Field(i)
		if field.IsExported() {
			exported[field.Name] = field.Type
		}
	}
	if len(exported) != 2 || exported["Updater"] == nil || exported["Manager"] == nil {
		t.Fatalf("unexpected exported Runtime fields: %v", exported)
	}
	stateStore := reflect.TypeOf((*state.Store)(nil))
	for name, fieldType := range exported {
		if fieldType == stateStore {
			t.Fatalf("exported authority escape %s: %v", name, fieldType)
		}
	}
}

func TestRuntimeCannotExposeLockedUpdaterLifecycle(t *testing.T) {
	managerType := reflect.TypeOf((*instance.Manager)(nil))
	for i := 0; i < managerType.NumMethod(); i++ {
		method := managerType.Method(i)
		if method.Name == "UpdaterLifecycle" {
			t.Fatalf("Manager still exposes raw locked lifecycle through %s", method.Name)
		}
	}
}
