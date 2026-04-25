// Generate TLA+ specs from Go runtime — ATOM-039 Formal CI Gate.
// Run: go run pkg/formal/generate.go
package main

import (
	"fmt"
	"os"

	"github.com/mahaasur13-sys/atom-runtime/pkg/formal"
)

func main() {
	gen := formal.New()

	specs := []struct {
		name string
		spec formal.RuntimeSpec
	}{
		{"EventStore", formal.BuildEventStoreSpec()},
		{"Snapshot", formal.BuildSnapshotSpec()},
		{"Byzantine", formal.BuildByzantineSpec()},
		{"Consensus", formal.BuildConsensusSpec()},
	}

	outputDir := "artifacts"
	if len(os.Args) > 1 {
		outputDir = os.Args[1]
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "create dir: %v\n", err)
		os.Exit(1)
	}

	for _, s := range specs {
		tla := gen.Generate(s.spec)
		path := outputDir + "/" + s.name + ".tla"
		if err := os.WriteFile(path, []byte(tla), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", s.name, err)
			os.Exit(1)
		}
		fmt.Printf("Generated: %s\n", path)
	}

	fmt.Println("All specs generated.")
}