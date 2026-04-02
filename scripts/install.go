package scripts

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pt/config"
)

// getDependencies reads metadata.json for a package and returns a list of
// unconditional dependency package names.
func getDependencies(packageName string, basePath string) ([]string, error) {
	metadataPath := filepath.Join(basePath, packageName, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}
	var response PyPIResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	var deps []string
	if requiresDist, ok := response.Info["requires_dist"]; ok && requiresDist != nil {
		if reqs, ok := requiresDist.([]interface{}); ok {
			for _, req := range reqs {
				if reqStr, ok := req.(string); ok {
					// Skip optional dependencies (extras, environment markers)
					if strings.Contains(reqStr, ";") || strings.Contains(reqStr, "[") {
						continue
					}
					deps = append(deps, parsePackageName(reqStr))
				}
			}
		}
	}
	return deps, nil
}

// getInstalledVersion returns the first version directory found for a package.
// If multiple versions exist, it returns the first (unsorted).
func getInstalledVersion(packageName string, basePath string) (string, error) {
	pkgDir := filepath.Join(basePath, packageName)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("no version directory found for package %s", packageName)
}

func Install(packageName string, version string) {
	visited := make(map[string]bool)
	installInternal(packageName, version, visited)
}

func installInternal(packageName string, version string, visited map[string]bool) {
	// Avoid infinite recursion
	if visited[packageName] {
		fmt.Printf("Package %s already processed, skipping to avoid cycles\n", packageName)
		return
	}
	visited[packageName] = true

	// Expand ~ in config.Path
	basePath := config.Path
	if strings.HasPrefix(basePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Could not get home directory: %v\n", err)
			return
		}
		basePath = filepath.Join(home, basePath[2:])
	}
	// Create directory: basePath/<package>/<version>/
	dir := filepath.Join(basePath, packageName, version)

	// Check if package version already exists
	if _, err := os.Stat(dir); err == nil {
		// Directory exists, check if it has content
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			fmt.Printf("Package %s version %s has not been installed", packageName, version)
			return
		}
	}
	// Read the directory entries
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	// Symlink directories to venv site-packages
	venvPath := GetVenvPath()
	if venvPath == "" {
		fmt.Println("VIRTUAL_ENV not set, skipping symlinking")
		return
	}
	// Find first entry in venvPath/lib/
	libPath := filepath.Join(venvPath, "lib")
	libEntries, err := os.ReadDir(libPath)
	if err != nil {
		fmt.Printf("Could not read lib directory: %v\n", err)
		return
	}
	if len(libEntries) == 0 {
		fmt.Println("No entries in lib directory")
		return
	}
	// Assume first entry is the python version directory
	sitePackagesPath := filepath.Join(libPath, libEntries[0].Name(), "site-packages")
	// Ensure site-packages exists
	if err := os.MkdirAll(sitePackagesPath, 0755); err != nil {
		fmt.Printf("Could not create site-packages directory: %v\n", err)
		return
	}
	// Symlink each directory in dir to site-packages
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		src := filepath.Join(dir, file.Name())
		dst := filepath.Join(sitePackagesPath, file.Name())
		// Check if destination already exists
		if _, err := os.Stat(dst); err == nil {
			fmt.Printf("Destination %s already exists, skipping\n", dst)
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			fmt.Printf("Failed to symlink %s to %s: %v\n", src, dst, err)
		} else {
			fmt.Printf("Symlinked %s -> %s\n", src, dst)
		}
	}

	// Process dependencies
	deps, err := getDependencies(packageName, basePath)
	if err != nil {
		fmt.Printf("Could not read dependencies for %s: %v\n", packageName, err)
		return
	}
	for _, dep := range deps {
		if dep == "" {
			continue
		}
		// Find installed version for dependency
		depVersion, err := getInstalledVersion(dep, basePath)
		if err != nil {
			fmt.Printf("Dependency %s not installed, skipping\n", dep)
			continue
		}
		fmt.Printf("Installing dependency: %s version %s\n", dep, depVersion)
		installInternal(dep, depVersion, visited)
	}
}
