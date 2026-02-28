package main

import (
	"fmt"

	"github.com/dusk-indust/decompose/internal/status"
)

func runStatus(projectRoot string, name string) error {
	if name != "" {
		return printSingleStatus(projectRoot, name)
	}
	return printAllStatuses(projectRoot)
}

func printSingleStatus(projectRoot, name string) error {
	ds := status.GetDecompositionStatus(projectRoot, name)
	fmt.Printf("Decomposition: %s\n\n", ds.Name)
	printStageTable(ds)
	return nil
}

func printAllStatuses(projectRoot string) error {
	decompositions, hasStage0 := status.ListDecompositions(projectRoot)

	if hasStage0 {
		fmt.Println("Stage 0: Development Standards  [complete]")
		fmt.Println()
	}

	if len(decompositions) == 0 && !hasStage0 {
		fmt.Println("No decompositions found.")
		fmt.Println("Run 'decompose <name>' to start a new decomposition.")
		return nil
	}

	for i, ds := range decompositions {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("Decomposition: %s\n", ds.Name)
		printStageTable(ds)
	}
	return nil
}

func printStageTable(ds status.DecompositionStatus) {
	for _, si := range ds.Stages {
		marker := "  "
		label := "pending"
		if si.Complete {
			marker = "  "
			label = "complete"
		}
		if si.Stage == ds.NextStage {
			marker = "->"
			label = "next"
		}

		fmt.Printf("  %s Stage %d: %-26s [%s]\n", marker, si.Stage, si.Name, label)
	}

	if ds.NextStage == -1 {
		fmt.Println("  All stages complete.")
	}
}
