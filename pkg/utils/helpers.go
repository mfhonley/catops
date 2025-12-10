package utils

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	constants "catops/config"
)

// FormatPercentage formats a float as percentage
func FormatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

// FormatNumber formats a number with proper spacing
func FormatNumber(value int64) string {
	return fmt.Sprintf("%d", value)
}

// ParseFloat parses a string to float64 with error handling
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// ParseInt parses a string to int64 with error handling
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// Round rounds a float64 to specified decimal places
func Round(value float64, decimals int) float64 {
	shift := math.Pow(10, float64(decimals))
	return math.Round(value*shift) / shift
}

// TruncateString truncates a string to specified length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// IsValidPercentage checks if a value is a valid percentage (0-100)
func IsValidPercentage(value float64) bool {
	return value >= 0 && value <= 100
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// FormatBytes formats bytes into human readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatKB formats kilobytes into human readable format
func FormatKB(kb int64) string {
	return FormatBytes(kb * 1024)
}

// FormatMemory formats memory usage with both percentage and absolute values
func FormatMemory(percent float64, usedKB, totalKB int64) string {
	if totalKB > 0 {
		return fmt.Sprintf("%.1f%% (%s / %s)", percent, FormatKB(usedKB), FormatKB(totalKB))
	}
	return fmt.Sprintf("%.1f%%", percent)
}

// FormatDisk formats disk usage with both percentage and absolute values
func FormatDisk(percent float64, usedKB, totalKB int64) string {
	if totalKB > 0 {
		return fmt.Sprintf("%.1f%% (%s / %s)", percent, FormatKB(usedKB), FormatKB(totalKB))
	}
	return fmt.Sprintf("%.1f%%", percent)
}

// FormatCPU formats CPU usage with both percentage and core information
func FormatCPU(percent float64, usedCores, totalCores int64) string {
	if totalCores > 0 {
		return fmt.Sprintf("%.1f%% (%d/%d cores)", percent, usedCores, totalCores)
	}
	return fmt.Sprintf("%.1f%%", percent)
}

// GetCurrentVersion gets CLI version from VERSION variable or version.txt
func GetCurrentVersion() string {
	// This will be set by main.go
	return "1.0.0" // Default fallback
}

// AddCLIHeaders adds required headers for new backend API
func AddCLIHeaders(req *http.Request, version string) {
	if req == nil {
		return
	}

	// Add required headers for new backend
	req.Header.Set("User-Agent", constants.HEADER_USER_AGENT)
	req.Header.Set(constants.HEADER_PLATFORM, runtime.GOOS)
	req.Header.Set(constants.HEADER_VERSION, version)
	req.Header.Set("Content-Type", "application/json")
}

// CreateCLIRequest creates HTTP request with proper CLI headers
func CreateCLIRequest(method, url string, body io.Reader, version string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	AddCLIHeaders(req, version)
	return req, nil
}
