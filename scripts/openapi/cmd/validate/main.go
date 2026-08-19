// Minimal OpenAPI validation (mirrors validate_openapi.py).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var httpMethods = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {},
	"options": {}, "head": {}, "trace": {},
}

var rootForbidden = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {},
}

func validateSpec(spec map[string]any) []string {
	var errors []string

	if spec["openapi"] != "3.0.3" {
		errors = append(errors, "Invalid or missing openapi version (expected 3.0.3)")
	}

	info, _ := spec["info"].(map[string]any)
	if info == nil || info["title"] == nil || info["version"] == nil {
		errors = append(errors, "Missing info.title or info.version")
	}

	paths, _ := spec["paths"].(map[string]any)
	if len(paths) == 0 {
		errors = append(errors, "Missing or empty paths")
		return errors
	}

	for key := range rootForbidden {
		if _, ok := spec[key]; ok {
			errors = append(errors, fmt.Sprintf("Root-level HTTP method key '%s' is forbidden", key))
		}
	}

	for path, rawItem := range paths {
		if rawItem == nil {
			errors = append(errors, fmt.Sprintf("Path item for '%s' must not be null", path))
			continue
		}
		pathItem, ok := rawItem.(map[string]any)
		if !ok {
			errors = append(errors, fmt.Sprintf("Path item for '%s' must be a mapping/object", path))
			continue
		}
		var operations []string
		for key := range pathItem {
			if _, isHTTP := httpMethods[key]; isHTTP {
				operations = append(operations, key)
			}
		}
		if len(operations) == 0 {
			errors = append(errors, fmt.Sprintf("Path '%s' must contain at least one HTTP operation", path))
			continue
		}
		for _, opKey := range operations {
			if _, ok := pathItem[opKey].(map[string]any); !ok {
				errors = append(errors, fmt.Sprintf("Operation '%s.%s' must be a mapping/object", path, opKey))
			}
		}
	}

	components, ok := spec["components"].(map[string]any)
	if !ok || len(components) == 0 {
		errors = append(errors, "Missing components section")
	}

	return errors
}

func validateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}

	var spec map[string]any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("invalid OpenAPI document: %w", err)
	}

	if errors := validateSpec(spec); len(errors) > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("OpenAPI validation failed for %s:\n", path))
		for _, msg := range errors {
			b.WriteString("  - " + msg + "\n")
		}
		return fmt.Errorf("%s", strings.TrimSpace(b.String()))
	}

	fmt.Printf("OpenAPI validation passed: %s\n", path)
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: validate <openapi.yaml>|--all\n")
		os.Exit(1)
	}

	if os.Args[1] == "--all" {
		root := findRoot()
		openapiDir := filepath.Join(root, "packages", "openapi")
		paths := []string{filepath.Join(openapiDir, "openapi.yaml")}
		matches, _ := filepath.Glob(filepath.Join(openapiDir, "*-service.yaml"))
		paths = append(paths, matches...)
		for _, path := range paths {
			if err := validateFile(path); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		return
	}

	if err := validateFile(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findRoot() string {
	wd, _ := os.Getwd()
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "packages", "openapi")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return wd
		}
	}
}
