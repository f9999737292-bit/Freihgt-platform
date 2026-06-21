// YAML to JSON converter for OpenAPI specs (fallback when Python/PyYAML is unavailable).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: yamltojson <input.yaml> <output.json>\n")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Input file not found: %s\n", inputPath)
		os.Exit(1)
	}

	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse YAML: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s\n", outputPath)
}
