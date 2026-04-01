/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"pt/scripts"
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
		if len(args) == 0 {
			fmt.Println("Usage: pt install <package> [version]")
			return
		}
		packageName := args[0]
		var version string
		if len(args) > 1 {
			version = args[1]
		} else {
			version = ""
		}
		resolvedVersion, err := scripts.Add(packageName, version, true)
		if err != nil {
			fmt.Printf("Failed to add package: %v\n", err)
			return
		}
		scripts.Install(packageName, resolvedVersion)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	// installCmd.Flags().BoolP(&recache, "r", false, "Recache PyPI response")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// installCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
