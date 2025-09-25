package main

import (
	"benchctl/internal"
	"flag"
	"fmt"
	"os"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to the configuration file")
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
	internal.RunWorkflow(cfg)
}
