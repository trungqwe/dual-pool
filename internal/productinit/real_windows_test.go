package productinit

import (
	"bytes"
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
	for _, p := range purposes {
		v, e := store.Get(p)
		if e != nil || !bytes.Equal(before[p], v) {
			secretstore.Zero(v)
			t.Fatal("key stability failed")
		}
		secretstore.Zero(v)
	}
	t.Logf("REAL_PRODUCT_INIT_PASS ready=%t key_count=%d second_run_unchanged=%t", second.Ready, len(before), true)
}
