package main

import (
	"benchctl/internal"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var cfgPath string
	var metadataFlags flagSlice
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to the configuration file")
	flag.Var(&metadataFlags, "metadata", `custom metadata in the format "key"="value" (can be used multiple times)`)
	flag.Parse()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Println("Error reading configuration file:", err)
		os.Exit(1)
	}

	cfg, err := internal.ParseYAML(data)
	if err != nil {
		fmt.Println("Error parsing configuration file:", err)
		os.Exit(1)
	}

	// Parse metadata flags into a map
	customMetadata := make(map[string]string)
	for _, metadataFlag := range metadataFlags {
		parts := strings.SplitN(metadataFlag, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Invalid metadata format: %s. Expected format: key=value\n", metadataFlag)
			os.Exit(1)
		}
		customMetadata[parts[0]] = parts[1]
	}

	internal.RunWorkflow(cfg, customMetadata)
}

// flagSlice implements flag.Value to handle multiple --metadata flags
type flagSlice []string

func (f *flagSlice) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *flagSlice) Set(value string) error {
	*f = append(*f, value)
	return nil
}
