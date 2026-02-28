package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dusk-indust/decompose/internal/export"
)

func runExport(projectRoot string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: decompose export <name>")
	}
	name := args[0]

	data, err := export.ExportDecomposition(projectRoot, name)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	_, err = os.Stdout.Write(append(out, '\n'))
	return err
}
