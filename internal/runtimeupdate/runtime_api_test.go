package runtimeupdate_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/trungqwe/dual-pool/internal/instance"
	"github.com/trungqwe/dual-pool/internal/runtimeupdate"
	"github.com/trungqwe/dual-pool/internal/state"
	"github.com/trungqwe/dual-pool/internal/upstreamcatalog"
	"github.com/trungqwe/dual-pool/internal/upstreamstage"
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

func TestComposeUpdaterDoesNotAcceptCallerSelectedAuthority(t *testing.T) {
	composition := reflect.TypeOf(instance.ComposeUpdater)
	if composition.NumIn() != 1 || composition.In(0) != reflect.TypeOf((*instance.Manager)(nil)) {
		t.Fatalf("unexpected ComposeUpdater public signature: %v", composition)
	}
}

func TestRuntimeConfigDoesNotExposeSmokeInjection(t *testing.T) {
	typeOfConfig := reflect.TypeOf(runtimeupdate.Config{})
	for i := 0; i < typeOfConfig.NumField(); i++ {
		if typeOfConfig.Field(i).Name == "Smoke" {
			t.Fatal("runtimeupdate.Config still exposes caller-selected Smoke")
		}
	}
}

func TestRuntimeCandidateMethodsExposeNoMutableReleaseAuthority(t *testing.T) {
	runtimeType := reflect.TypeOf((*runtimeupdate.Runtime)(nil))
	stage, ok := runtimeType.MethodByName("StageVerifiedCandidate")
	if !ok || stage.Type.NumIn() != 2 || stage.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() || stage.Type.NumOut() != 2 || stage.Type.Out(0) != reflect.TypeOf(upstreamstage.Result{}) {
		t.Fatalf("unsafe StageVerifiedCandidate signature: %v", stage)
	}
	install, ok := runtimeType.MethodByName("InstallVerifiedCandidate")
	if !ok || install.Type.NumIn() != 3 || install.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() || install.Type.In(2) != reflect.TypeOf(upstreamstage.Result{}) || install.Type.NumOut() != 3 {
		t.Fatalf("unsafe InstallVerifiedCandidate signature: %v", install)
	}
	forbidden := []reflect.Type{
		reflect.TypeOf(""), reflect.TypeOf(upstreamcatalog.Provenance{}), reflect.TypeOf((*upstreamcatalog.Catalog)(nil)),
		reflect.TypeOf((*upstreamstage.Downloader)(nil)).Elem(), reflect.TypeOf((*upstreamstage.Verifier)(nil)).Elem(),
	}
	for _, method := range []reflect.Method{stage, install} {
		for i := 0; i < method.Type.NumIn(); i++ {
			for _, denied := range forbidden {
				if method.Type.In(i) == denied {
					t.Fatalf("%s exposes caller-selected authority %v", method.Name, denied)
				}
			}
		}
	}
}
