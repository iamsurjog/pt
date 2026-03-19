/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net/http"
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
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type PyPIResponse struct {
	Releases map[string][]FileInfo  `json:"releases"`
	Info     map[string]interface{} `json:"info"` 
}

func run(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: provide package name as arg")
		return
	}
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", args[0])
	fmt.Println(url)

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

	
	info := response.Info
	name, ok := info["name"].(string)
	if !ok {
		fmt.Println("No 'name' field")
		return
	}
	version, _ := info["version"].(string)                   
	fmt.Printf("Package: %s (version: %s)\n", name, version) 

	for version, files := range response.Releases {
		if len(files) == 0 {
			continue 
		}

		fmt.Printf("Version %s (%d files):\n", version, len(files))
		for _, file := range files {
			fmt.Printf("  - %s: %s\n", file.Filename, file.URL)
		}
		fmt.Println()
	}
	fmt.Printf(config.Path)

	// fmt.Println("Available versions:")
	// for version := range response.Releases {
	// 	fmt.Println(version) // "0.0.1", "0.0.2", "0.0.3", etc.
	// }
}
