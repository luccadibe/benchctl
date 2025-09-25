package main

import (
	"os"
)

type DataType int

const (
	// allowed data types for the csv output
	DataTypeInt64 DataType = iota
	DataTypeFloat64
	DataTypeString
	DataTypeTimestamp
)

type Config struct {
	// the benchmark name to be used in metadata
	Name string
	// the directory to save the results
	OutputDir string
	// hosts is a map of host aliases to host details
	Hosts map[string]Host
	// the data schema for the csv output
	DataSchema map[string]DataType
	// the stages to be executed
	Stages []Stage
	// the plots to be generated
	Plots []Plot
}

// Host is a host in the benchmark. It can be a remote host or the local host. If its remote, commands will be executed using ssh.
type Host struct {
	IP          string
	Username    string
	Password    string
	KeyFile     string
	KeyPassword string
}

// Stage is a stage in the benchmark. It can be the execution of a certain command or script on a host. It may include copying some output data from a host to another (usually local).
type Stage struct {
	Name string
	// must be one of host aliases or local
	Host    string
	Command string
	// at least one of command or script needs to be provided. Cannot be both.
	// Script is a path to the script to execute. It will be copied to the host and executed.
	Script      string
	HealthCheck HealthCheck
	// Optional
	Outputs []Output
}

// HealthCheck is a health check to determine if the stage is executed successfully. (Optional)
type HealthCheck struct {
	Type    string
	Target  string
	Timeout string
	Retries int
}

// Evaluate evaluates the health check. Returns nil if the health check is successful, otherwise returns an error.
func (h *HealthCheck) Evaluate() error {
	return nil
}

// Output is a file to collect after the stage is executed. (Optional)
type Output struct {
	// Remote path is the path to the file on the host.
	RemotePath string
	// Local path is the path to save the file on the local machine.
	// If not provided, the file will be saved in the output_dir for the benchmark run. like ./results/run-001/some_file.txt
	LocalPath string
}

// For now unimplemented.
type Plot struct {
	Name  string
	Title string

	Format      string
	ExportPath  string
	X           string
	Y           string
	Aggregation string
	Columns     []string
}

func main() {
	os.Exit(0)
}
