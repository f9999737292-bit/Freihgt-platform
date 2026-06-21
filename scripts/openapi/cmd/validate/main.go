// Minimal OpenAPI validation (mirrors validate_openapi.py).
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: validate <openapi.yaml>\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "File not found: %s\n", os.Args[1])
		os.Exit(1)
	}

	var spec map[string]any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid OpenAPI document: %v\n", err)
		os.Exit(1)
	}

	if spec["openapi"] != "3.0.3" {
		fmt.Fprintln(os.Stderr, "Invalid or missing openapi version (expected 3.0.3)")
		os.Exit(1)
	}

	info, _ := spec["info"].(map[string]any)
	if info == nil || info["title"] == nil || info["version"] == nil {
		fmt.Fprintln(os.Stderr, "Missing info.title or info.version")
		os.Exit(1)
	}

	paths, _ := spec["paths"].(map[string]any)
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "Missing or empty paths")
		os.Exit(1)
	}

	components, ok := spec["components"].(map[string]any)
	if !ok || len(components) == 0 {
		fmt.Fprintln(os.Stderr, "Missing components section")
		os.Exit(1)
	}

	fmt.Println("OpenAPI validation passed")
}
