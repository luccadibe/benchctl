package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareRunMetadata compares two run metadata and returns a list of differences
func CompareRunMetadata(metadata1, metadata2 *RunMetadata) ([]ComparisonResult, error) {
	var results []ComparisonResult

	// Get all unique keys from both metadata
	keys := getAllKeys(metadata1.Custom, metadata2.Custom)

	for _, key := range keys {
		v1, exists1 := metadata1.Custom[key]
		v2, exists2 := metadata2.Custom[key]

		// Try to parse both values as floats
		f1, isFloat1 := parseFloat(v1)
		f2, isFloat2 := parseFloat(v2)

		// If both are floats, create float comparison
		if isFloat1 && isFloat2 {
			results = append(results, &FloatComparisonResult{
				Key:    key,
				Value1: f1,
				Value2: f2,
			})
		} else {
			// Otherwise, create string comparison
			results = append(results, &StringComparisonResult{
				Key:    key,
				Value1: getStringPtr(v1, exists1),
				Value2: getStringPtr(v2, exists2),
			})
		}
	}

	return results, nil
}

func PrintComparisonResults(results []ComparisonResult) string {
	out := strings.Builder{}
	for _, result := range results {
		out.WriteString(result.Format() + "\n")
	}
	return out.String()
}

// parseFloat attempts to parse a string as a float64
func parseFloat(s string) (*float64, bool) {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &f, true
	}
	return nil, false
}

// getStringPtr returns a pointer to the string if it exists, nil otherwise
func getStringPtr(s string, exists bool) *string {
	if exists {
		return &s
	}
	return nil
}

type ComparisonResult interface {
	GetKey() string
	GetValue1() string
	GetValue2() string
	Format() string
}

type StringComparisonResult struct {
	Key    string
	Value1 *string
	Value2 *string
}

func (r *StringComparisonResult) GetKey() string {
	return r.Key
}

func (r *StringComparisonResult) GetValue1() string {
	if r.Value1 == nil {
		return ""
	}
	return *r.Value1
}

func (r *StringComparisonResult) GetValue2() string {
	if r.Value2 == nil {
		return ""
	}
	return *r.Value2
}

func (r *StringComparisonResult) Format() string {
	v1 := r.GetValue1()
	v2 := r.GetValue2()
	if v1 == "" {
		return fmt.Sprintf("%s: (missing) -> %s", r.Key, v2)
	}
	if v2 == "" {
		return fmt.Sprintf("%s: %s -> (missing)", r.Key, v1)
	}
	return fmt.Sprintf("%s: %s -> %s", r.Key, v1, v2)
}

type FloatComparisonResult struct {
	Key    string
	Value1 *float64
	Value2 *float64
}

func (r *FloatComparisonResult) GetKey() string {
	return r.Key
}

func (r *FloatComparisonResult) GetValue1() string {
	if r.Value1 == nil {
		return ""
	}
	return fmt.Sprintf("%f", *r.Value1)
}

func (r *FloatComparisonResult) GetValue2() string {
	if r.Value2 == nil {
		return ""
	}
	return fmt.Sprintf("%f", *r.Value2)
}

func (r *FloatComparisonResult) Format() string {
	v1 := r.GetValue1()
	v2 := r.GetValue2()
	if v1 == "" {
		return fmt.Sprintf("%s: (missing) -> %s", r.Key, v2)
	}
	if v2 == "" {
		return fmt.Sprintf("%s: %s -> (missing)", r.Key, v1)
	}

	// Calculate percentage change if both values exist
	if r.Value1 != nil && r.Value2 != nil && *r.Value1 != 0 {
		change := ((*r.Value2 - *r.Value1) / *r.Value1) * 100
		return fmt.Sprintf("%s: %s -> %s (%.1f%% change)", r.Key, v1, v2, change)
	}
	return fmt.Sprintf("%s: %s -> %s", r.Key, v1, v2)
}

// getAllKeys returns all unique keys from both maps
func getAllKeys(m1, m2 map[string]string) []string {
	keySet := make(map[string]bool)

	// Add keys from first map
	for k := range m1 {
		keySet[k] = true
	}

	// Add keys from second map
	for k := range m2 {
		keySet[k] = true
	}

	// Convert set to slice
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	return keys
}
