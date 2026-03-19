/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

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

	"github.com/spf13/cobra"

	"pt/config"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Installing packages: ", args)
		run(args)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// installCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

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

func run(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: pt install <package> [version]")
		return
	}
	packageName := args[0]
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP error: %s\n", resp.Status)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Reading body failed: %v\n", err)
		return
	}
	var response PyPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("JSON unmarshal failed: %v\n", err)
		return
	}
	// Resolve version
	var version string
	if len(args) >= 2 {
		version = args[1]
	} else {
		version, _ = response.Info["version"].(string)
	}
	if version == "" {
		fmt.Println("Could not determine version")
		return
	}
	// Check version exists in releases
	files, ok := response.Releases[version]
	if !ok {
		fmt.Printf("Version %s not found for package %s\n", version, packageName)
		return
	}
	if len(files) == 0 {
		fmt.Printf("Version %s has no downloadable files\n", version)
		return
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Failed to create directory %s: %v\n", dir, err)
		return
	}
	// Download the file
	fmt.Printf("Downloading %s...\n", selected.Filename)
	fileResp, err := http.Get(selected.URL)
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		return
	}
	defer fileResp.Body.Close()
	destPath := filepath.Join(dir, selected.Filename)
	destFile, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("Failed to create file %s: %v\n", destPath, err)
		return
	}
	defer destFile.Close()
	if _, err := io.Copy(destFile, fileResp.Body); err != nil {
		fmt.Printf("Failed to write file: %v\n", err)
		return
	}

	fmt.Printf("Extracting %s...\n", selected.Filename)
	if err := extractFile(destPath, dir); err != nil {
		fmt.Printf("Extraction failed: %v\n", err)
		return
	}
	fmt.Println("Done.")

	fmt.Printf("Saved %s to %s\n", selected.Filename, dir)

	fmt.Println(config.Path)
	fmt.Println(response.Info["version"])

}
