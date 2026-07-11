package memory

import (
	"context"
	"testing"
)

func TestExternalStoreScopesOwnership(t *testing.T) {
	s := NewExternalStore()
	item, _, err := s.Write(context.Background(), ExternalRecord{AppID: "fund", OwnerID: "u1", Scope: "profile", Kind: "summary", Summary: "balanced"})
	if err != nil {
		t.Fatal(err)
	}
	got, trace, err := s.Query(context.Background(), "fund", "u1", "profile", 10)
	if err != nil || len(got) != 1 || got[0].ID != item.ID || trace.Operation != "memory_query" {
		t.Fatalf("got=%#v trace=%#v err=%v", got, trace, err)
	}
	other, _, err := s.Query(context.Background(), "fund", "u2", "profile", 10)
	if err != nil || len(other) != 0 {
		t.Fatal("ownership leaked")
	}
}

func BenchmarkExternalStoreQuery(b *testing.B) {
	store := NewExternalStore()
	for index := 0; index < 100; index++ {
		_, _, err := store.Write(context.Background(), ExternalRecord{AppID: "app", OwnerID: "owner", Scope: "profile", Kind: "summary", Summary: "A bounded summary."})
		if err != nil {
			b.Fatalf("seed write: %v", err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, _, err := store.Query(context.Background(), "app", "owner", "profile", 20); err != nil {
			b.Fatalf("query: %v", err)
		}
	}
}
