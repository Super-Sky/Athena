package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"moss/internal/controlplane"
)

// main builds one compiled system-asset package from the current truth dir.
// main 会基于当前 truth dir 构建一份编译后的 system asset 包。
func main() {
	truthDir := flag.String("truth-dir", filepath.Join("config", "system", "truth"), "system truth directory")
	stateDir := flag.String("state-dir", filepath.Join("output", "system-state"), "generated active system state directory")
	storePath := flag.String("store-path", filepath.Join("config", "controlplane", "build-system-assets", "overrides.json"), "control-plane override store path")
	outputDir := flag.String("output-dir", filepath.Join("output", "system-assets"), "compiled system asset package output directory")
	flag.Parse()

	manager := controlplane.NewManagerWithTruthAndStateDirs(controlplane.NewFileStore(*storePath), *truthDir, *stateDir)
	manifest, err := manager.BuildCompiledAssetsPackage(context.Background(), *outputDir)
	if err != nil {
		log.Fatalf("build compiled system assets failed: %v", err)
	}
	fmt.Printf("built %d assets into %s (truth dir version: %s)\n", manifest.AssetCount, *outputDir, manifest.TruthDirVersion)
	for _, item := range manifest.Assets {
		fmt.Printf("%s\t%s\t%s\n", item.AssetID, item.AssetType, item.PackageFile)
	}
}
