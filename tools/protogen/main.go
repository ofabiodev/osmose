package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root, err := findRoot()
	if err == nil {
		err = generate(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(root string) error {
	sourceDir := filepath.Join(root, "schema")
	files, err := filepath.Glob(filepath.Join(sourceDir, "*.proto"))
	if err != nil {
		return fmt.Errorf("find schema files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no schema files found in %s", sourceDir)
	}
	sort.Strings(files)

	module, err := modulePath(root)
	if err != nil {
		return err
	}
	outputDir := filepath.Join(root, "proto")
	entries, err := os.ReadDir(outputDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read generated directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := os.RemoveAll(filepath.Join(outputDir, entry.Name())); err != nil {
				return fmt.Errorf("remove generated package %s: %w", entry.Name(), err)
			}
		}
	}

	args := []string{
		"--proto_path=" + sourceDir,
		"--go_out=" + root,
		"--go_opt=module=" + module,
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".proto")
		args = append(args, "--go_opt=M"+name+".proto="+module+"/proto/"+name)
	}
	args = append(args, files...)
	command := exec.Command("protoc", args...)
	command.Dir = root
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("protoc: %w", err)
	}

	generated, err := filepath.Glob(filepath.Join(outputDir, "*", "*.go"))
	if err != nil {
		return err
	}
	for _, file := range generated {
		if err := format(file); err != nil {
			return err
		}
	}
	return nil
}

func findRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("go.mod not found")
		}
		root = parent
	}
}

func modulePath(root string) (string, error) {
	command := exec.Command("go", "list", "-m")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("go list -m: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func format(path string) error {
	command := exec.Command("gofmt", "-w", path)
	command.Stdout = io.Discard
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("gofmt %s: %w", path, err)
	}
	return nil
}
