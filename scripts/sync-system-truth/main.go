package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"moss/internal/controlplane"
)

// main syncs markdown sources under one truth dir into system-managed compiled assets.
// main 会把 truth dir 下的 markdown 主源同步编译成 system-managed 资产产物。
func main() {
	truthDir := flag.String("truth-dir", filepath.Join("config", "system", "truth"), "system truth directory")
	stateDir := flag.String("state-dir", filepath.Join("output", "system-state"), "generated active system state directory")
	storePath := flag.String("store-path", filepath.Join("config", "controlplane", "sync-system-truth", "overrides.json"), "control-plane override store path")
	flag.Parse()

	manager := controlplane.NewManagerWithTruthAndStateDirs(controlplane.NewFileStore(*storePath), *truthDir, *stateDir)
	if err := manager.SyncSystemSources(context.Background()); err != nil {
		log.Fatalf("sync system sources failed: %v", err)
	}
	items, err := manager.ListSystemResources(context.Background())
	if err != nil {
		log.Fatalf("list system resources failed: %v", err)
	}
	for _, item := range items {
		detail, err := manager.GetSystemResource(context.Background(), item.AssetID)
		if err != nil {
			log.Fatalf("get system resource %s failed: %v", item.AssetID, err)
		}
		fmt.Printf("%s\t%s\t%s\n", item.AssetID, item.AssetType, detail.SourcePath)
	}
}
