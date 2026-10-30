package db

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSetSettingsAtomic(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()
	prefix := fmt.Sprintf("test.atomic_settings.%d", time.Now().UnixNano())
	defer func() { _, _ = d.Exec(`DELETE FROM settings WHERE key LIKE $1`, prefix+".%") }()
	values := map[string]string{prefix + ".worker_pools": "[]", prefix + ".workers": "3"}
	if err := d.SetSettingsAtomic(context.Background(), values); err != nil {
		t.Fatal(err)
	}
	for key, want := range values {
		got, ok, err := d.GetSetting(key)
		if err != nil || !ok || got != want {
			t.Fatalf("key %q: value=%q ok=%v err=%v, want %q", key, got, ok, err, want)
		}
	}
}

func TestIntSettings(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()
	key := fmt.Sprintf("test.int_settings.%d", time.Now().UnixNano())
	defer func() { _, _ = d.Exec(`DELETE FROM settings WHERE key=$1`, key) }()

	if got := d.GetInt(key, 6000); got != 6000 {
		t.Fatalf("unset GetInt=%d, want default 6000", got)
	}
	if err := d.SetInt(key, 8000); err != nil {
		t.Fatal(err)
	}
	if got := d.GetInt(key, 6000); got != 8000 {
		t.Fatalf("GetInt=%d, want 8000", got)
	}
	if err := d.SetSetting(key, "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if got := d.GetInt(key, 6000); got != 6000 {
		t.Fatalf("unparseable GetInt=%d, want default 6000", got)
	}
}
