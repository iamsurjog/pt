package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("HELLO THERE")
	dirPath := "." // Current directory
	this, err := os.Getwd()
	fmt.Printf("CWD: %s\n", this)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Listing directory %s:\n", dirPath)
	for _, entry := range entries {
		fmt.Printf(" - %s (Is Directory: %v)\n", entry.Name(), entry.IsDir())
	}
}

