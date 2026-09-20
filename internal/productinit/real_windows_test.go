package productinit

import (
	"crypto/subtle"
	"os"
	"testing"

	"github.com/trungqwe/dual-pool/internal/secretstore"
)

func TestRealProductInitialization(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_REAL_PRODUCT_INIT") != "1" {
		t.Skip("explicit gate is closed")
	}
	i, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	first, err := i.Initialize()
	if err != nil || !first.Ready {
		t.Fatalf("first initialization failed: %v", err)
	}
	state, err := InspectCurrent()
	if err != nil || !state.Ready || !state.RootExists || state.DirectoriesReady != 7 || state.KeysPresent != 4 {
		t.Fatal("post-initialization inspection failed")
	}
	store := secretstore.New()
	before := map[secretstore.Purpose][]byte{}
	for _, p := range purposes {
		v, e := store.Get(p)
		if e != nil || len(v) != 32 {
			t.Fatal("key verification failed")
		}
		before[p] = v
	}
	defer zeroMap(before)
	second, err := i.Initialize()
	if err != nil || !second.Ready || second.CreatedDirectories != 0 || second.CreatedKeys != 0 {
		t.Fatalf("idempotence failed: %v", err)
	}
	state, err = InspectCurrent()
	if err != nil || !state.Ready || state.DirectoriesReady != 7 || state.KeysPresent != 4 {
		t.Fatal("second inspection failed")
	}
	for _, dir := range i.directories()[1:] {
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			t.Fatal("unexpected product artifact")
		}
	}
	for _, p := range purposes {
		v, e := store.Get(p)
		if e != nil || len(v) != len(before[p]) || subtle.ConstantTimeCompare(before[p], v) != 1 {
			secretstore.Zero(v)
			t.Fatal("key stability failed")
		}
		secretstore.Zero(v)
	}
	t.Logf("REAL_PRODUCT_INIT_PASS ready=%t key_count=%d second_run_unchanged=%t", second.Ready, len(before), true)
}
