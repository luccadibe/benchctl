package internal

import (
	"encoding/csv"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gonumplot "gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// Default plot settings
const (
	DefaultWidth    = 4 * vg.Inch
	DefaultHeight   = 4 * vg.Inch
	DefaultBins     = 16
	DefaultBoxWidth = 20
)

var (
	// Default colors for plots
	DefaultFillColor = color.RGBA{127, 188, 165, 1}
	DefaultLineColor = color.RGBA{255, 0, 0, 255}

	// Common timestamp formats to try
	TimestampFormats = []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05.000000000Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05.000000000",
		"2006-01-02",
		"15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	}
)

// CSVData represents parsed CSV data with column mapping
type CSVData struct {
	Headers []string
	Rows    [][]string
	XCol    int
	YCol    int
}

// parseTimestamp attempts to parse a timestamp string using multiple formats
func parseTimestamp(ts string) (time.Time, error) {
	for _, format := range TimestampFormats {
		if t, err := time.Parse(format, ts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}

// parseCSVData parses CSV data and maps columns
func parseCSVData(csvData [][]string, xCol, yCol string) (*CSVData, error) {
	if len(csvData) < 2 {
		return nil, errors.New("CSV data must have at least header and one data row")
	}

	headers := csvData[0]
	xIdx := -1
	yIdx := -1

	// Find column indices
	for i, header := range headers {
		if header == xCol {
			xIdx = i
		}
		if header == yCol && yCol != "" {
			yIdx = i
		}
	}

	if xIdx == -1 {
		return nil, fmt.Errorf("x column '%s' not found", xCol)
	}
	if yCol != "" && yIdx == -1 {
		return nil, fmt.Errorf("y column '%s' not found", yCol)
	}

	return &CSVData{
		Headers: headers,
		Rows:    csvData[1:],
		XCol:    xIdx,
		YCol:    yIdx,
	}, nil
}

// parseFloatValue parses a string to float64, handling common formats
func parseFloatValue(s string) (float64, error) {
	// Try parsing as float first
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	// Try parsing as integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return float64(i), nil
	}

	return 0, fmt.Errorf("unable to parse as number: %s", s)
}

func GeneratePlot(plot Plot, dataSourcePath, exportPath string) error {
	data, err := os.ReadFile(dataSourcePath)
	if err != nil {
		return err
	}

	csvData, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		return err
	}

	parsedData, err := parseCSVData(csvData, plot.X, plot.Y)
	if err != nil {
		return err
	}

	p := gonumplot.New()
	p.Title.Text = plot.Title
	p.X.Label.Text = plot.X
	p.Y.Label.Text = plot.Y

	switch plot.Type {
	case "time_series":
		err = createTimeSeriesPlot(p, parsedData, plot)
	case "histogram":
		err = createHistogramPlot(p, parsedData, plot)
	case "boxplot":
		err = createBoxPlot(p, parsedData, plot)
	default:
		return fmt.Errorf("unsupported plot type: %s", plot.Type)
	}

	if err != nil {
		return err
	}

	// Ensure export directory exists
	exportDir := filepath.Dir(exportPath)
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err
	}

	return p.Save(DefaultWidth, DefaultHeight, exportPath)
}

// createTimeSeriesPlot creates a time series plot from CSV data
func createTimeSeriesPlot(p *gonumplot.Plot, data *CSVData, plot Plot) error {
	points := make(plotter.XYs, 0, len(data.Rows))

	for _, row := range data.Rows {
		if len(row) <= data.XCol || len(row) <= data.YCol {
			continue // Skip incomplete rows
		}

		// Parse X value (timestamp)
		var x float64
		if t, err := parseTimestamp(row[data.XCol]); err == nil {
			x = float64(t.Unix())
		} else {
			// If not a timestamp, try parsing as number
			if f, err := parseFloatValue(row[data.XCol]); err == nil {
				x = f
			} else {
				continue // Skip invalid rows
			}
		}

		// Parse Y value
		y, err := parseFloatValue(row[data.YCol])
		if err != nil {
			continue // Skip invalid rows
		}

		points = append(points, plotter.XY{X: x, Y: y})
	}

	if len(points) == 0 {
		return errors.New("no valid data points found")
	}

	line, err := plotter.NewLine(points)
	if err != nil {
		return err
	}
	line.Color = DefaultLineColor
	p.Add(line)

	return nil
}

// createHistogramPlot creates a histogram from CSV data
func createHistogramPlot(p *gonumplot.Plot, data *CSVData, plot Plot) error {
	values := make(plotter.Values, 0, len(data.Rows))

	for _, row := range data.Rows {
		if len(row) <= data.XCol {
			continue // Skip incomplete rows
		}

		// Parse X value for histogram
		x, err := parseFloatValue(row[data.XCol])
		if err != nil {
			continue // Skip invalid rows
		}

		values = append(values, x)
	}

	if len(values) == 0 {
		return errors.New("no valid data points found")
	}

	hist, err := plotter.NewHist(values, DefaultBins)
	if err != nil {
		return err
	}
	hist.FillColor = DefaultFillColor
	hist.Normalize(1) // Normalize to sum to 1
	p.Add(hist)

	return nil
}

// createBoxPlot creates a box plot from CSV data
func createBoxPlot(p *gonumplot.Plot, data *CSVData, plot Plot) error {
	// Group values by X column (categorical data)
	groups := make(map[string][]float64)

	for _, row := range data.Rows {
		if len(row) <= data.XCol || len(row) <= data.YCol {
			continue // Skip incomplete rows
		}

		xVal := row[data.XCol]
		if xVal == "" {
			continue // Skip empty X values
		}

		// Parse Y value for box plot
		y, err := parseFloatValue(row[data.YCol])
		if err != nil {
			continue // Skip invalid rows
		}

		groups[xVal] = append(groups[xVal], y)
	}

	if len(groups) == 0 {
		return errors.New("no valid data points found")
	}

	// Create box plots for each group
	boxWidth := vg.Points(DefaultBoxWidth)
	offset := 0.0

	for _, values := range groups {
		if len(values) == 0 {
			continue
		}

		box, err := plotter.NewBoxPlot(boxWidth, offset, plotter.Values(values))
		if err != nil {
			return err
		}
		box.FillColor = DefaultFillColor

		p.Add(box)

		// Space out multiple box plots
		offset += 1.0
	}

	return nil
}
