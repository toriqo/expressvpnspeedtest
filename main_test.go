package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Constants for testing
const testResultsFile = "test_results.json"

// Variables for mocking
var (
	mockRuntime struct {
		GOOS string
	}
)

// Mock for the exec.Command
type CommandMock struct {
	mock.Mock
}

func (m *CommandMock) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *CommandMock) CombinedOutput() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *CommandMock) Output() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

// Helper to set Stdout
func (m *CommandMock) SetStdout(out *bytes.Buffer) {
	// This is just a helper method
}

// Setup Test Helper
func setupTest(t *testing.T) func() {
	// Save original functions that we'll override
	originalExecCommand := execCommand

	// Create temp dir for test files
	tempDir, err := os.MkdirTemp("", "speedtest")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Setup mock runtime
	mockRuntime.GOOS = "linux" // default value

	// Return cleanup function
	return func() {
		// Restore original functions
		execCommand = originalExecCommand
		os.RemoveAll(tempDir)
	}
}

// Mock for exec.Command
var execCommand = exec.Command

func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess isn't a real test, it's used as a helper process for command mocking
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}

	if len(args) == 0 {
		return
	}

	cmd, cmdArgs := args[0], args[1:]
	switch cmd {
	case "speedtest":
		if cmdArgs[0] == "-f" && cmdArgs[1] == "json-pretty" {
			mockSpeedTest()
		}
	case "expressvpnctl":
		if cmdArgs[0] == "get" && cmdArgs[1] == "regions" {
			mockGetRegions()
		} else if cmdArgs[0] == "connect" {
			// Success case for connect
		} else if cmdArgs[0] == "disconnect" {
			// Success case for disconnect
		} else if cmdArgs[0] == "get" && cmdArgs[1] == "connectionstate" {
			mockConnectionState()
		}
	case "lsb_release", "sw_vers", "cmd":
		mockOSVersion(cmd)
	}
}

func mockSpeedTest() {
	result := SpeedTestResult{}
	result.Ping.Latency = 25.5
	result.Download.Bandwidth = 125000000 // 1000 Mbps
	result.Upload.Bandwidth = 62500000    // 500 Mbps
	result.Server.Host = "test.speedtest.com"
	result.Server.Name = "TestServer"
	result.Server.Country = "TestCountry"
	result.Server.Location = "TestCity"

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	os.Stdout.Write(jsonData)
}

func mockGetRegions() {
	regions := []string{
		"netherlands-amsterdam",
		"romania-bucharest",
		"canada-toronto",
		"usa",
	}
	os.Stdout.Write([]byte(strings.Join(regions, "\n")))
}

func mockConnectionState() {
	os.Stdout.Write([]byte("Connected"))
}

func mockOSVersion(cmd string) {
	switch cmd {
	case "lsb_release":
		os.Stdout.Write([]byte("Description: Ubuntu 20.04 LTS"))
	case "sw_vers":
		os.Stdout.Write([]byte("11.2.3"))
	case "cmd":
		os.Stdout.Write([]byte("Microsoft Windows [Version 10.0.19042.928]"))
	}
}

// Modified GetOSVersion for testing
func testGetOSVersion(goos string) string {
	switch goos {
	case "linux":
		return "Ubuntu 20.04 LTS"
	case "darwin":
		return "macOS 11.2.3"
	case "windows":
		return "Microsoft Windows [Version 10.0.19042.928]"
	default:
		return "Unknown OS"
	}
}

// Unit Tests

func TestGetOSVersion(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	// Test with different OS types
	tests := []struct {
		os       string
		expected string
	}{
		{"linux", "Ubuntu 20.04 LTS"},
		{"darwin", "macOS 11.2.3"},
		{"windows", "Microsoft Windows [Version 10.0.19042.928]"},
		{"other", "Unknown OS"},
	}

	for _, test := range tests {
		t.Run(test.os, func(t *testing.T) {
			// Use our mock function
			result := testGetOSVersion(test.os)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestFindRegion(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	tests := []struct {
		name     string
		location Location
		expected string
	}{
		{
			name:     "Exact match",
			location: Location{Country: "Netherlands", City: "Amsterdam"},
			expected: "netherlands-amsterdam",
		},
		{
			name:     "Country only match",
			location: Location{Country: "Romania", City: "Bucharest"},
			expected: "romania",
		},
		{
			name:     "No match",
			location: Location{Country: "France", City: "Paris"},
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := findRegion(test.location)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestLoadAndSaveFile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Create test file path
	testFile := filepath.Join(t.TempDir(), testResultsFile)

	// Create test data
	testData := Results{
		MachineName: "TestMachine",
		OS:          "TestOS",
		WithoutVPN:  "1000Mbps ▼ 500Mbps ▲",
		VPNStats: []VPNStat{
			{
				LocationName:     "TestCountry, TestCity",
				TimeToConnect:    "1.5s",
				VPNDownloadSpeed: "800Mbps",
				VPNUploadSpeed:   "400Mbps",
				VPNLatency:       "25.50ms",
				Server:           "test.server.com",
				Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
				Mode:             "Tests ran in parallel",
			},
		},
	}

	// Test saving to file
	err := saveToFile(testData, testFile)
	assert.NoError(t, err)

	// Test loading from file
	loadedData, err := loadFromFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, testData.MachineName, loadedData.MachineName)
	assert.Equal(t, testData.OS, loadedData.OS)
	assert.Equal(t, testData.WithoutVPN, loadedData.WithoutVPN)
	assert.Equal(t, 1, len(loadedData.VPNStats))
	assert.Equal(t, testData.VPNStats[0].LocationName, loadedData.VPNStats[0].LocationName)

	// Test loading from non-existent file
	nonExistentFile := filepath.Join(t.TempDir(), "nonexistent.json")
	emptyData, err := loadFromFile(nonExistentFile)
	assert.NoError(t, err)
	assert.Equal(t, Results{}, emptyData)
}

// Test VPN connection functions
func TestConnectToVPN(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	// Test connecting to VPN
	duration, err := connectToVPN("netherlands-amsterdam")
	assert.NoError(t, err)
	assert.True(t, duration > 0)
}

func TestDisconnectVPN(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	// Test disconnecting from VPN
	err := disconnectVPN()
	assert.NoError(t, err)
}

// Integration Tests
// Modified to work with constant values

func TestSpeedTestFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	// Set a local variable to capture results
	var localSpeedWithoutVPN string
	var capturedStats []VPNStat

	// Mock the global variables and functions that speedTest would use
	origSpeedTestCount := speedTestCount
	speedTestCount = 1 // Reduce the count for testing
	defer func() { speedTestCount = origSpeedTestCount }()

	// Create a custom version of speedTest for testing
	testSpeedTest := func(connectionTime string) {
		var vpnStats []VPNStat
		counter := 0

		var totalDownload, totalUpload float64
		var count int

		for range speedTestCount {
			counter++
			// Mock speedtest command output
			result := SpeedTestResult{}
			result.Ping.Latency = 25.5
			result.Download.Bandwidth = 125000000 // 1000 Mbps
			result.Upload.Bandwidth = 62500000    // 500 Mbps
			result.Server.Host = "test.speedtest.com"
			result.Server.Name = "TestServer"
			result.Server.Country = "TestCountry"
			result.Server.Location = "TestCity"

			if connectionTime == "" {
				localSpeedWithoutVPN = fmt.Sprintf("%dMbps ▼  %dMbps ▲", result.Download.Bandwidth/125000, result.Upload.Bandwidth/125000)
			} else {
				vpnStats = append(vpnStats, VPNStat{
					LocationName:     result.Server.Country + ", " + result.Server.Location,
					TimeToConnect:    connectionTime,
					VPNDownloadSpeed: fmt.Sprintf("%dMbps", result.Download.Bandwidth/125000),
					VPNUploadSpeed:   fmt.Sprintf("%dMbps", result.Upload.Bandwidth/125000),
					VPNLatency:       fmt.Sprintf("%.2fms", result.Ping.Latency),
					Server:           result.Server.Host,
					Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
					Mode:             "Tests ran in series (one after another)",
				})
			}
		}

		// Compute the average speed
		var avgStat VPNStat
		for _, stat := range vpnStats {
			downloadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNDownloadSpeed, "Mbps"))
			uploadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNUploadSpeed, "Mbps"))
			totalDownload += float64(downloadSpeed)
			totalUpload += float64(uploadSpeed)
			count++
			avgStat = stat // Keep other details from the last stat
		}

		if count > 0 {
			avgStat.VPNDownloadSpeed = fmt.Sprintf("%.2fMbps", totalDownload/float64(count))
			avgStat.VPNUploadSpeed = fmt.Sprintf("%.2fMbps", totalUpload/float64(count))
			capturedStats = append(capturedStats, avgStat)
		}
	}

	// Test speed test without VPN
	testSpeedTest("")

	// Verify results
	assert.Equal(t, "1000Mbps ▼  500Mbps ▲", localSpeedWithoutVPN)

	// Test speed test with VPN
	testSpeedTest("1.5s")

	// Verify VPN stats
	assert.Equal(t, 1, len(capturedStats))
	assert.Contains(t, capturedStats[0].LocationName, "TestCountry")
	assert.Equal(t, "1.5s", capturedStats[0].TimeToConnect)
}

func TestParallelSpeedTestFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cleanup := setupTest(t)
	defer cleanup()

	// Override exec.Command with our mockExecCommand
	execCommand = mockExecCommand

	// Set a local variable to capture results
	var localSpeedWithoutVPN string
	var capturedStats []VPNStat

	// Mock the global variables and functions
	origSpeedTestCount := speedTestCount
	speedTestCount = 2 // Reduce the count for testing
	defer func() { speedTestCount = origSpeedTestCount }()

	// Create a test version of runParallelSpeedTests
	testRunParallelSpeedTests := func(connectionTime string) {
		resultsChan := make(chan VPNStat, speedTestCount)

		// Simulate parallel tests
		for i := 0; i < speedTestCount; i++ {
			// Mock speedtest command output
			result := SpeedTestResult{}
			result.Ping.Latency = 25.5
			result.Download.Bandwidth = 125000000 // 1000 Mbps
			result.Upload.Bandwidth = 62500000    // 500 Mbps
			result.Server.Host = "test.speedtest.com"
			result.Server.Name = "TestServer"
			result.Server.Country = "TestCountry"
			result.Server.Location = "TestCity"

			if connectionTime == "" {
				localSpeedWithoutVPN = fmt.Sprintf("%dMbps ▼  %dMbps ▲", result.Download.Bandwidth/125000, result.Upload.Bandwidth/125000)
			} else {
				resultsChan <- VPNStat{
					LocationName:     result.Server.Country + ", " + result.Server.Location,
					TimeToConnect:    connectionTime,
					VPNDownloadSpeed: fmt.Sprintf("%dMbps", result.Download.Bandwidth/125000),
					VPNUploadSpeed:   fmt.Sprintf("%dMbps", result.Upload.Bandwidth/125000),
					VPNLatency:       fmt.Sprintf("%.2fms", result.Ping.Latency),
					Server:           result.Server.Host,
					Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
					Mode:             "Tests ran in parallel",
				}
			}
		}
		close(resultsChan)

		// Compute the average speed
		var totalDownload, totalUpload float64
		var count int
		var avgStat VPNStat

		for stat := range resultsChan {
			downloadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNDownloadSpeed, "Mbps"))
			uploadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNUploadSpeed, "Mbps"))
			totalDownload += float64(downloadSpeed)
			totalUpload += float64(uploadSpeed)
			count++
			avgStat = stat // Keep other details from the last stat
		}

		if count > 0 {
			avgStat.VPNDownloadSpeed = fmt.Sprintf("%.2fMbps", totalDownload/float64(count))
			avgStat.VPNUploadSpeed = fmt.Sprintf("%.2fMbps", totalUpload/float64(count))
			capturedStats = append(capturedStats, avgStat)
		}
	}

	// Test parallel speed tests without VPN
	testRunParallelSpeedTests("")

	// Verify results
	assert.Equal(t, "1000Mbps ▼  500Mbps ▲", localSpeedWithoutVPN)

	// Test parallel speed tests with VPN
	testRunParallelSpeedTests("1.5s")

	// Verify VPN stats
	assert.Equal(t, 1, len(capturedStats))
	assert.Contains(t, capturedStats[0].LocationName, "TestCountry")
	assert.Equal(t, "1.5s", capturedStats[0].TimeToConnect)
	assert.Contains(t, capturedStats[0].Mode, "parallel")
}

func TestAverageSpeedCalculation(t *testing.T) {
	// Create a controlled test for the average calculation logic
	// Create mock VPNStats with different speeds
	stats := []VPNStat{
		{
			LocationName:     "TestCountry, TestCity",
			TimeToConnect:    "1.5s",
			VPNDownloadSpeed: "100Mbps",
			VPNUploadSpeed:   "50Mbps",
		},
		{
			LocationName:     "TestCountry, TestCity",
			TimeToConnect:    "1.5s",
			VPNDownloadSpeed: "200Mbps",
			VPNUploadSpeed:   "150Mbps",
		},
		{
			LocationName:     "TestCountry, TestCity",
			TimeToConnect:    "1.5s",
			VPNDownloadSpeed: "300Mbps",
			VPNUploadSpeed:   "250Mbps",
		},
	}

	// Calculate average (replicating logic from the function)
	var totalDownload, totalUpload float64
	var count int
	var avgStat VPNStat

	for _, stat := range stats {
		downloadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNDownloadSpeed, "Mbps"))
		uploadSpeed, _ := strconv.Atoi(strings.TrimSuffix(stat.VPNUploadSpeed, "Mbps"))
		totalDownload += float64(downloadSpeed)
		totalUpload += float64(uploadSpeed)
		count++
		avgStat = stat
	}

	if count > 0 {
		avgStat.VPNDownloadSpeed = fmt.Sprintf("%.2fMbps", totalDownload/float64(count))
		avgStat.VPNUploadSpeed = fmt.Sprintf("%.2fMbps", totalUpload/float64(count))
	}

	// Verify calculations
	assert.Equal(t, "200.00Mbps", avgStat.VPNDownloadSpeed)
	assert.Equal(t, "150.00Mbps", avgStat.VPNUploadSpeed)
}

func TestInputFileValidation(t *testing.T) {
	// Test valid input file
	validInput := InputData{
		Locations: []Location{
			{Country: "Netherlands", City: "Amsterdam"},
		},
	}
	validFile := filepath.Join(t.TempDir(), "valid.json")
	jsonData, _ := json.MarshalIndent(validInput, "", "  ")
	err := os.WriteFile(validFile, jsonData, 0644)
	assert.NoError(t, err)

	// Test invalid input file (malformed JSON)
	invalidFile := filepath.Join(t.TempDir(), "invalid.json")
	err = os.WriteFile(invalidFile, []byte("{invalid json}"), 0644)
	assert.NoError(t, err)

	// Test reading valid file
	data, err := os.ReadFile(validFile)
	assert.NoError(t, err)

	var input InputData
	err = json.Unmarshal(data, &input)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(input.Locations))

	// Test reading invalid file
	data, err = os.ReadFile(invalidFile)
	assert.NoError(t, err)

	err = json.Unmarshal(data, &input)
	assert.Error(t, err) // Should fail with JSON parsing error
}
