package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	calculatedfields "github.com/vittorioparagallo/pocketbase-calculated-fields-plugin"

	"github.com/pocketbase/pocketbase"
)


var dataDirFlag = flag.String("data", "", "data directory")

func main() {
		flag.Parse()

	dataDir := *dataDirFlag
	if dataDir == "" {
		root := findRepoRoot()
		dataDir = filepath.Join(root, "tests", "pb_data")
	}

	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: dataDir,
	})
	// init plugin (o chiami calculatedfields.Bind... se non passi da xpb)
	if err := (&calculatedfields.Plugin{}).Init(app); err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

	
}
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	for {
		// marker: go.mod (puoi aggiungere anche .git se vuoi)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("repo root not found (go.mod missing)")
		}
		dir = parent
	}
}