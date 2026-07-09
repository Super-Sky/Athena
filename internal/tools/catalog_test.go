package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func TestCatalogRegisterSnapshotAndDelete(t *testing.T) {
	catalog := NewCatalog(nil)
	definition := Definition{Name: "remote_lookup", BaseTool: catalogTestTool{name: "remote_lookup"}}
	if err := catalog.Register(definition); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	snapshot := catalog.Snapshot()
	if len(snapshot) != 1 || snapshot["remote_lookup"].Name != "remote_lookup" {
		t.Fatalf("snapshot = %#v, want remote_lookup", snapshot)
	}
	delete(snapshot, "remote_lookup")
	if _, ok := catalog.Get("remote_lookup"); !ok {
		t.Fatal("mutating snapshot changed catalog")
	}
	if !catalog.Delete("remote_lookup") {
		t.Fatal("Delete() = false, want true")
	}
	if _, ok := catalog.Get("remote_lookup"); ok {
		t.Fatal("deleted tool remains in catalog")
	}
}

func TestCatalogSupportsConcurrentSnapshotsAndReplacement(t *testing.T) {
	catalog := NewCatalog(nil)
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < 100; index++ {
				if err := catalog.Register(Definition{Name: "remote_lookup", BaseTool: catalogTestTool{name: "remote_lookup"}}); err != nil {
					t.Errorf("Register() error = %v", err)
					return
				}
				_ = catalog.Snapshot()
			}
		}()
	}
	wait.Wait()
}

type catalogTestTool struct {
	name string
}

func (t catalogTestTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name}, nil
}

func (t catalogTestTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	return `{}`, nil
}
