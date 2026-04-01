package scripts

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pt/config"
)

func Install(packageName string, version string) {
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
}
