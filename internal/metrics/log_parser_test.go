package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestLogParserWithSamples tests the parser against real-world log samples
// Run with: go test -v -run TestLogParserWithSamples ./internal/metrics/
func TestLogParserWithSamples(t *testing.T) {
	parser := NewLogParser()

	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("LOG PARSER TEST RESULTS")
	fmt.Println(strings.Repeat("=", 100))

	var passed, failed int
	var issues []string

	for _, sample := range TestLogSamples {
		entry := parser.ParseLogLine(sample.Log)

		fmt.Printf("\n--- %s ---\n", sample.Name)
		fmt.Printf("INPUT:   %.100s", sample.Log)
		if len(sample.Log) > 100 {
			fmt.Printf("...")
		}
		fmt.Println()

		// Print what was extracted
		fmt.Printf("PARSED:\n")
		fmt.Printf("  Level:      %s\n", entry.Level)
		fmt.Printf("  Message:    %.80s", entry.Message)
		if len(entry.Message) > 80 {
			fmt.Printf("...")
		}
		fmt.Println()

		if entry.TraceID != "" {
			fmt.Printf("  TraceID:    %s\n", entry.TraceID)
		}
		if entry.SpanID != "" {
			fmt.Printf("  SpanID:     %s\n", entry.SpanID)
		}
		if entry.RequestID != "" {
			fmt.Printf("  RequestID:  %s\n", entry.RequestID)
		}
		if entry.UserID != "" {
			fmt.Printf("  UserID:     %s\n", entry.UserID)
		}
		if entry.HTTPMethod != "" {
			fmt.Printf("  HTTP:       %s %s -> %d\n", entry.HTTPMethod, entry.HTTPPath, entry.HTTPStatus)
		}
		if entry.HTTPDuration > 0 {
			fmt.Printf("  Duration:   %dms\n", entry.HTTPDuration)
		}
		if entry.ErrorType != "" {
			fmt.Printf("  Error:      %s\n", entry.ErrorType)
		}
		if entry.StackTrace != "" {
			lines := strings.Split(entry.StackTrace, "\n")
			fmt.Printf("  Stack:      %s", lines[0])
			if len(lines) > 1 {
				fmt.Printf(" (+%d lines)", len(lines)-1)
			}
			fmt.Println()
		}
		if entry.SourceIP != "" {
			fmt.Printf("  SourceIP:   %s\n", entry.SourceIP)
		}
		if entry.UserAgent != "" {
			fmt.Printf("  UserAgent:  %.50s\n", entry.UserAgent)
		}
		if len(entry.Attributes) > 0 {
			attrJSON, _ := json.Marshal(entry.Attributes)
			fmt.Printf("  Attrs:      %.80s", string(attrJSON))
			if len(attrJSON) > 80 {
				fmt.Printf("...")
			}
			fmt.Println()
		}

		// Check expected values
		var sampleIssues []string

		if expected, ok := sample.Expected["level"]; ok {
			if entry.Level != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("level: got '%s', want '%s'", entry.Level, expected))
			}
		}

		if expected, ok := sample.Expected["message"]; ok {
			expectedStr := expected.(string)
			if !strings.Contains(entry.Message, expectedStr) {
				sampleIssues = append(sampleIssues, fmt.Sprintf("message: missing '%s'", expectedStr))
			}
		}

		if expected, ok := sample.Expected["http_method"]; ok {
			if entry.HTTPMethod != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("http_method: got '%s', want '%s'", entry.HTTPMethod, expected))
			}
		}

		if expected, ok := sample.Expected["http_path"]; ok {
			if entry.HTTPPath != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("http_path: got '%s', want '%s'", entry.HTTPPath, expected))
			}
		}

		if expected, ok := sample.Expected["http_status"]; ok {
			expectedInt := expected.(int)
			if entry.HTTPStatus != expectedInt {
				sampleIssues = append(sampleIssues, fmt.Sprintf("http_status: got %d, want %d", entry.HTTPStatus, expectedInt))
			}
		}

		if expected, ok := sample.Expected["trace_id"]; ok {
			if entry.TraceID != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("trace_id: got '%s', want '%s'", entry.TraceID, expected))
			}
		}

		if expected, ok := sample.Expected["request_id"]; ok {
			if entry.RequestID != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("request_id: got '%s', want '%s'", entry.RequestID, expected))
			}
		}

		if expected, ok := sample.Expected["user_id"]; ok {
			if entry.UserID != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("user_id: got '%s', want '%s'", entry.UserID, expected))
			}
		}

		if expected, ok := sample.Expected["source_ip"]; ok {
			if entry.SourceIP != expected {
				sampleIssues = append(sampleIssues, fmt.Sprintf("source_ip: got '%s', want '%s'", entry.SourceIP, expected))
			}
		}

		if expected, ok := sample.Expected["error_type"]; ok {
			expectedStr := expected.(string)
			if !strings.Contains(entry.ErrorType, expectedStr) {
				sampleIssues = append(sampleIssues, fmt.Sprintf("error_type: got '%s', want contains '%s'", entry.ErrorType, expectedStr))
			}
		}

		if expected, ok := sample.Expected["stack_trace"]; ok {
			expectedStr := expected.(string)
			if !strings.Contains(entry.StackTrace, expectedStr) {
				sampleIssues = append(sampleIssues, fmt.Sprintf("stack_trace: missing '%s'", expectedStr))
			}
		}

		if len(sampleIssues) > 0 {
			failed++
			fmt.Printf("  ❌ ISSUES:\n")
			for _, issue := range sampleIssues {
				fmt.Printf("     - %s\n", issue)
				issues = append(issues, fmt.Sprintf("[%s] %s", sample.Name, issue))
			}
		} else {
			passed++
			fmt.Printf("  ✅ OK\n")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Printf("SUMMARY: %d passed, %d failed out of %d tests\n", passed, failed, len(TestLogSamples))

	if len(issues) > 0 {
		fmt.Println("\nALL ISSUES:")
		for _, issue := range issues {
			fmt.Printf("  • %s\n", issue)
		}
	}

	fmt.Println(strings.Repeat("=", 100))

	if failed > 0 {
		t.Errorf("%d tests failed", failed)
	}
}

// TestSingleLog allows testing a single log line interactively
// Run with: go test -v -run TestSingleLog ./internal/metrics/
func TestSingleLog(t *testing.T) {
	parser := NewLogParser()

	// Change this line to test specific logs
	testLog := `{"level":"error","message":"Failed to process request","error":"NullPointerException","stack_trace":"at com.app.Service.process(Service.java:42)\n  at com.app.Controller.handle(Controller.java:15)","request_id":"req-456"}`

	entry := parser.ParseLogLine(testLog)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("SINGLE LOG TEST")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("INPUT: %s\n\n", testLog)

	// Pretty print the result
	result, _ := json.MarshalIndent(entry, "", "  ")
	fmt.Printf("PARSED:\n%s\n", string(result))
	fmt.Println(strings.Repeat("=", 80))
}

// BenchmarkLogParser benchmarks the parser performance
func BenchmarkLogParser(b *testing.B) {
	parser := NewLogParser()

	// Mix of different log formats
	logs := []string{
		`{"timestamp":"2025-01-15T10:30:00Z","level":"info","message":"Request completed","method":"POST","path":"/api/users","status":201}`,
		`time="2025-01-15T10:30:00Z" level=error msg="Connection failed" error="timeout"`,
		`192.168.1.100 - - [15/Jan/2025:10:30:00 +0000] "GET /index.html HTTP/1.1" 200 2326`,
		`2025-01-15 10:30:00,123 - myapp - ERROR - Database connection failed`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ParseLogLine(logs[i%len(logs)])
	}
}
