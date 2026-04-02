package scripts

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pt/config"
)

type FileInfo struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	Packagetype string `json:"packagetype"`
}

type PyPIResponse struct {
	Releases map[string][]FileInfo  `json:"releases"`
	Info     map[string]interface{} `json:"info"`
}

func extractFile(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".whl") {
		return extractZip(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}
func extractZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		outPath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		outPath := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(outPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(outPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, tr)
			outFile.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Add(packageName string, version string, fast bool) (string, error) {
	visited := make(map[string]bool)
	return addInternal(packageName, version, fast, visited)
}

func addInternal(packageName string, version string, fast bool, visited map[string]bool) (string, error) {
	// Avoid infinite recursion
	if visited[packageName] {
		fmt.Printf("Package %s already processed, skipping to avoid cycles\n", packageName)
		return "", nil
	}
	visited[packageName] = true

	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP error: %s\n", resp.Status)
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Reading body failed: %v\n", err)
		return "", err
	}
	var response PyPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("JSON unmarshal failed: %v\n", err)
		return "", err
	}
	// Print requirements from requires_dist
	var deps []string
	if requiresDist, ok := response.Info["requires_dist"]; ok && requiresDist != nil {
		if reqs, ok := requiresDist.([]interface{}); ok {
			fmt.Println("Requirements:")
			for _, req := range reqs {
				if reqStr, ok := req.(string); ok {
					fmt.Println("  -", reqStr)
					// Skip dependencies with environment markers (e.g., extras, python version)
					if strings.Contains(reqStr, ";") || strings.Contains(reqStr, "[") {
						continue
					}
					deps = append(deps, parsePackageName(reqStr))
				}
			}
		}
	}
	// Resolve version
	if version == "" {
		version, _ = response.Info["version"].(string)
	}

	// Check version exists in releases
	files, ok := response.Releases[version]
	if !ok {
		fmt.Printf("Version %s not found for package %s\n", version, packageName)
		return "", fmt.Errorf("version %s not found", version)
	}
	if len(files) == 0 {
		fmt.Printf("Version %s has no downloadable files\n", version)
		return "", fmt.Errorf("version %s has no downloadable files", version)
	}
	// Pick wheel first, fall back to any file
	var selected *FileInfo
	for i := range files {
		if files[i].Packagetype == "bdist_wheel" {
			selected = &files[i]
			break
		}
	}
	if selected == nil {
		selected = &files[0]
	}
	// Expand ~ in config.Path
	expandedBasePath := config.Path
	if strings.HasPrefix(expandedBasePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Could not get home directory: %v\n", err)
			return "", err
		}
		expandedBasePath = filepath.Join(home, expandedBasePath[2:])
	}
	// Save metadata.json in package directory
	packageDir := filepath.Join(expandedBasePath, packageName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		fmt.Printf("Failed to create package directory %s: %v\n", packageDir, err)
		return "", err
	}
	metadataPath := filepath.Join(packageDir, "metadata.json")
	if err := os.WriteFile(metadataPath, body, 0644); err != nil {
		fmt.Printf("Failed to write metadata.json: %v\n", err)
		return "", err
	}
	// Create directory: expandedBasePath/<package>/<version>/
	dir := filepath.Join(expandedBasePath, packageName, version)

	// Check if package version already exists
	alreadyInstalled := false
	if _, err := os.Stat(dir); err == nil {
		// Directory exists, check if it has content
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) > 0 {
			fmt.Printf("Package %s version %s already installed at %s\n", packageName, version, dir)
			alreadyInstalled = true
		}
	}

	if !alreadyInstalled {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("Failed to create directory %s: %v\n", dir, err)
			return "", err
		}
		// Download the file
		fmt.Printf("Downloading %s...\n", selected.Filename)
		fileResp, err := http.Get(selected.URL)
		if err != nil {
			fmt.Printf("Download failed: %v\n", err)
			return "", err
		}
		defer fileResp.Body.Close()
		destPath := filepath.Join(dir, selected.Filename)
		destFile, err := os.Create(destPath)
		if err != nil {
			fmt.Printf("Failed to create file %s: %v\n", destPath, err)
			return "", err
		}
		defer destFile.Close()
		if _, err := io.Copy(destFile, fileResp.Body); err != nil {
			fmt.Printf("Failed to write file: %v\n", err)
			return "", err
		}

		fmt.Printf("Extracting %s...\n", selected.Filename)
		if err := extractFile(destPath, dir); err != nil {
			fmt.Printf("Extraction failed: %v\n", err)
			return "", err
		}
		fmt.Println("Done.")

		fmt.Printf("Saved %s to %s\n", selected.Filename, dir)
	}

	// Process dependencies
	for _, dep := range deps {
		if dep != "" {
			fmt.Printf("Adding dependency: %s\n", dep)
			if _, err := addInternal(dep, "", fast, visited); err != nil {
				fmt.Printf("Failed to add dependency %s: %v\n", dep, err)
				// Continue with other dependencies
			}
		}
	}

	return version, nil
}
