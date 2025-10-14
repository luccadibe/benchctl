package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goforj/godump"
)

func InspectRun(runPath string, verbose bool) string {
	// find metadata.json in the runPath
	metadataPath := filepath.Join(runPath, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "Error reading run metadatafile: " + err.Error()
	}
	var runmd RunMetadata
	err = json.Unmarshal(data, &runmd)
	if err != nil {
		return "Error unmarshalling run metadata: " + err.Error()
	}

	out := strings.Builder{}
	out.WriteString("Start time: " + runmd.StartTime.Format(time.RFC3339) + "\n")
	out.WriteString("End time: " + runmd.EndTime.Format(time.RFC3339) + "\n")
	out.WriteString(fmt.Sprintf("Run custom metadata: \n%+v", stringifyCustomMetadata(runmd.Custom)+"\n"))

	if verbose {
		out.WriteString(fmt.Sprintf("Run config: %+v", godump.DumpStr(runmd.Config)+"\n"))
	}
	return out.String()
}

func stringifyCustomMetadata(customMd map[string]string) string {
	out := strings.Builder{}
	for key, value := range customMd {
		out.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	}
	return out.String()
}

func AddMetadata(runPath string, extraMetadata map[string]string) error {

	// find metadata.json in the runPath
	metadataPath := filepath.Join(runPath, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("error reading run metadatafile: %w", err)
	}
	var runmd RunMetadata
	err = json.Unmarshal(data, &runmd)
	if err != nil {
		return fmt.Errorf("error unmarshalling run metadata: %w", err)
	}
	for key, value := range extraMetadata {
		runmd.Custom[key] = value
	}
	metadataBytes, err := json.MarshalIndent(runmd, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling run metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return fmt.Errorf("error writing run metadata: %w", err)
	}
	return nil
}
