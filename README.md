# ExpressVPN Speed Test Tool

## Table of Contents
- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [Command Line Options](#command-line-options)
- [Input Format](#input-format)
- [Output Format](#output-format)
- [Implementation Details](#implementation-details)
- [Data Structures](#data-structures)
- [Core Functions](#core-functions)
- [Utility Functions](#utility-functions)
- [File Operations](#file-operations)
- [VPN Management](#vpn-management)
- [Error Handling](#error-handling)
- [Concurrency Model](#concurrency-model)
- [Performance Considerations](#performance-considerations)
- [Examples](#examples)
- [Dependencies](#dependencies)
- [Troubleshooting](#troubleshooting)

## Overview

ExpressVPN Speed Test is a command-line utility for benchmarking VPN connection performance across multiple ExpressVPN server locations. The tool allows users to:

- Measure baseline internet speed without VPN connection
- Automatically connect to multiple ExpressVPN locations defined in an input file
- Measure VPN connection establishment time for each location
- Run multiple speed tests for each connection (in parallel or series)
- Aggregate results and calculate average performance metrics
- Save comprehensive performance data to a structured JSON file

This utility helps users identify the optimal ExpressVPN servers for their location, track VPN performance over time, and compare speeds with and without VPN connections.

## Installation

### Prerequisites

- Go 1.16 or later
- ExpressVPN client installed and configured
  - The `expressvpnctl` command must be available in your PATH
  - Valid ExpressVPN subscription and activated account
- Speedtest CLI installed
  - The `speedtest` command must be available in your PATH
- Internet connection

### Building from Source

```bash
# Clone or download the source code
git clone https://github.com/toriqo/expressvpnspeedtest.git
cd expressvpnspeedtest

# Build the executable
go build -o expressvpnspeedtest

# Optional: Make it executable (Linux/macOS)
chmod +x expressvpnspeedtest

# Optional: Move to a directory in your PATH
sudo mv expressvpnspeedtest /usr/local/bin/
```

### Dependencies

The tool depends on the following external Go package:
- `github.com/pterm/pterm` - Terminal output formatting and progress indicators

## Usage

Basic usage involves providing a JSON file containing the VPN locations you want to test:

```bash
expressvpnspeedtest locations.json
```

## Command Line Options

```
expressvpnspeedtest [options] <input_file.json>
```

Available options:

- `-h` - Display help menu and usage instructions
- `-s` - Run speed tests in series (one after another) instead of in parallel
  - Useful for high-bandwidth connections (e.g., 1Gbps) where parallel tests might interfere with each other
- `-r N` - Set the number of speed tests per VPN location (default: 5)
  - When used with `-s`, runs N tests in sequence
  - When used without `-s`, runs N tests in parallel

Examples:

```bash
# Run with default settings (5 parallel speed tests per location)
expressvpnspeedtest locations.json

# Run with 10 parallel speed tests per location
expressvpnspeedtest -r 10 locations.json

# Run 3 sequential speed tests per location
expressvpnspeedtest -s -r 3 locations.json

# Display help menu
expressvpnspeedtest -h
```

## Input Format

The program requires a JSON input file specifying the VPN locations to test. Each location must include a country name and optionally a city name:

```json
{
  "locations": [
    {
      "country": "Netherlands",
      "city": "Amsterdam"
    },
    {
      "country": "Romania",
      "city": "Bucharest"
    },
    {
      "country": "Canada",
      "city": "Toronto"
    },
    {
      "country": "USA"
    }
  ]
}
```

Notes:
- Country and city names must match ExpressVPN's naming conventions
- If city is omitted, the program will attempt to connect to any server in the specified country
- Case sensitivity matters for matching ExpressVPN regions

## Output Format

Results are saved to `results-TIMESTAMP.json` in the current working directory. This file has the following structure:

```json
{
  "MachineName": "your-computer-hostname",
  "OS": "operating system: version",
  "WithoutVPN": "100Mbps ▼ 20Mbps ▲",
  "VPNStats": [
    {
      "LocationName": "Netherlands, Amsterdam",
      "TimeToConnect": "1.234s",
      "VPNDownloadSpeed": "85.50Mbps",
      "VPNUploadSpeed": "15.75Mbps",
      "VPNLatency": "45.20ms",
      "Server": "speedtest-server.example.com",
      "Date/Time": "2025-03-03 14:25:30",
      "Mode": "Tests ran in parallel"
    },
    {
      "LocationName": "Romania, Bucharest",
      "TimeToConnect": "2.345s",
      "VPNDownloadSpeed": "75.25Mbps",
      "VPNUploadSpeed": "18.50Mbps",
      "VPNLatency": "65.30ms",
      "Server": "speedtest-server2.example.com",
      "Date/Time": "2025-03-03 14:30:45",
      "Mode": "Tests ran in series (one after another)"
    }
  ]
}
```

Field descriptions:
- `MachineName`: Hostname of the test machine
- `OS`: Operating system name and version
- `WithoutVPN`: Baseline speed without VPN (download ▼ upload ▲)
- `VPNStats`: Array of test results containing:
  - `LocationName`: VPN location (country, city)
  - `TimeToConnect`: Time taken to establish VPN connection
  - `VPNDownloadSpeed`: Average measured download speed
  - `VPNUploadSpeed`: Average measured upload speed
  - `VPNLatency`: Connection latency to speedtest server
  - `Server`: Speedtest server hostname used for testing
  - `Date/Time`: Timestamp when the test was performed
  - `Mode`: Whether tests ran in parallel or in series

## Implementation Details

### Operation Flow

The tool follows a sequential process:
<ol type="1">
  <li>Parse command-line options and input file</li>
  <li>Run baseline speed test without VPN (either in parallel or series based on flags)</li>
  <li>For each location in the input file:
    <ol type="a">
      <li>Find matching ExpressVPN region</li>
      <li>Connect to the VPN and measure connection time</li>
      <li>Run speed tests (in parallel or series based on flags)</li>
      <li>Calculate average performance metrics</li>
      <li>Save results</li>
      <li>Disconnect from VPN</li>
    </ol>
  </li>
  <li>All results are saved to a structured JSON file</li>
</ol>

## Data Structures

### Location
```go
type Location struct {
    Country string `json:"country"`
    City    string `json:"city"`
}
```
Represents a VPN location to test, with country and optional city.

### InputData
```go
type InputData struct {
    Locations []Location `json:"locations"`
}
```
Structure for parsing the input JSON file containing locations to test.

### Results
```go
type Results struct {
    MachineName string    `json:"MachineName"`
    OS          string    `json:"OS"`
    WithoutVPN  string    `json:"WithoutVPN"`
    VPNStats    []VPNStat `json:"VPNStats"`
}
```
Structure for the output JSON file with test results.

### VPNStat
```go
type VPNStat struct {
    LocationName     string `json:"LocationName"`
    TimeToConnect    string `json:"TimeToConnect"`
    VPNDownloadSpeed string `json:"VPNDownloadSpeed"`
    VPNUploadSpeed   string `json:"VPNUploadSpeed"`
    VPNLatency       string `json:"VPNLatency"`
    Server           string `json:"Server"`
    Timestamp        string `json:"Date/Time"`
    Mode             string `json:"Mode"`
}
```
Individual VPN connection test result with performance metrics.

### SpeedTestResult
```go
type SpeedTestResult struct {
    Ping struct {
        Latency float64 `json:"latency"`
    } `json:"ping"`
    Download struct {
        Bandwidth int64 `json:"bandwidth"`
    } `json:"download"`
    Upload struct {
        Bandwidth int64 `json:"bandwidth"`
    } `json:"upload"`
    Server struct {
        Host     string `json:"host"`
        Name     string `json:"name"`
        Country  string `json:"country"`
        Location string `json:"location"`
    } `json:"server"`
}
```
Structure for parsing the JSON output from Speedtest CLI.

## Core Functions

### main()
The entry point of the program. Parses command-line arguments, reads the input file, and coordinates the testing process.

### speedTest(connectionTime string)
Runs sequential speed tests for a connection:
- Performs multiple tests one after another
- Each test uses the Speedtest CLI
- Collects performance metrics
- Calculates average values
- Used when the `-s` flag is provided

### runParallelSpeedTests(connectionTime string)
Runs concurrent speed tests for a connection:
- Launches multiple goroutines to run tests in parallel
- Uses channels to collect results
- Calculates average performance metrics
- Used by default or when the `-s` flag is not provided

## Utility Functions

### GetOSVersion() string
Detects the operating system version based on the runtime environment:
- For Linux: Uses `lsb_release -d`
- For macOS: Uses `sw_vers -productVersion`
- For Windows: Uses `cmd /C ver`
- Returns a formatted string with OS name and version

### findRegion(location Location) string
Maps a user-provided location to the corresponding ExpressVPN region:
- Retrieves available regions from ExpressVPN
- Attempts to match with both "country-city" and "country" formats
- Returns the matching region name or empty string if not found

### displayHelp()
Shows usage instructions and examples when the `-h` flag is used.

## File Operations

### writeToFile(newStats VPNStat)
Thread-safely updates the results JSON file with new test results:
- Uses mutex locking to prevent concurrent file access issues
- Creates a new results file if none exists
- Appends new test statistics to existing results
- Gets system information if this is the first write

### loadFromFile(fileName string) (Results, error)
Loads existing results from the JSON file:
- Reads and parses the file
- Returns empty results structure if file doesn't exist
- Returns error if file exists but can't be read or parsed

### saveToFile(data Results, fileName string) error
Writes results structure to JSON file:
- Pretty-prints the JSON with indentation
- Creates or overwrites the specified file
- Returns error if writing fails

## VPN Management

### getCommandOutput() ([]string, error)
Executes the `expressvpnctl get regions` command to retrieve available VPN regions.

### connectToVPN(region string) (time.Duration, error)
Connects to the specified VPN region:
- Measures the time taken to establish connection
- Returns the connection duration and any error
- Automatically waits for connection to be established

### disconnectVPN() error
Disconnects from the current VPN connection using the `expressvpnctl disconnect` command.

### waitForConnection()
Polls the VPN connection state until successfully connected:
- Checks connection status periodically
- Returns only when connection is established
- Prevents tests from running before connection is ready

## Error Handling

The tool implements several error handling mechanisms:
- Validates command-line arguments and flags
- Verifies input file existence and format
- Checks for VPN connection success/failure
- Handles speedtest execution errors
- Reports file operation failures
- Skips locations that don't match any available VPN regions

## Concurrency Model

The program uses Go's concurrency primitives:
- Goroutines: Used for parallel speed testing
- Channels: Collect results from concurrent tests
- WaitGroups: Ensure all tests complete before proceeding
- Mutex: Protects shared resources during file operations

## Performance Considerations

The tool offers two testing modes to accommodate different network environments:
1. **Parallel Mode (Default)**:
   - Runs multiple speed tests simultaneously
   - More efficient for most connections
   - May lead to resource contention on very high-speed connections

2. **Series Mode** (using `-s` flag):
   - Runs tests sequentially
   - Recommended for gigabit connections
   - More accurate for very high bandwidth networks
   - Takes longer to complete

## Examples

### Basic Usage

Test three default locations with default settings:
```bash
expressvpnspeedtest locations.json
```

Where `locations.json` contains:
```json
{
  "locations": [
    {"country": "USA", "city": "NewYork"},
    {"country": "UK", "city": "London"},
    {"country": "Japan", "city": "Tokyo"}
  ]
}
```

### Testing on High-Speed Network

For gigabit connections, use sequential testing:
```bash
expressvpnspeedtest -s locations.json
```

### Increasing Test Count

For more statistical accuracy, increase the number of tests:
```bash
expressvpnspeedtest -r 10 locations.json
```

## Unit & Integration Testing

Install Testify

```
go get github.com/stretchr/testify
```

then run
```
go test -v
```

## Dependencies

The tool has the following dependencies:
- Go standard library packages:
  - `bytes`: Buffer operations for command output
  - `encoding/json`: JSON parsing and formatting
  - `flag`: Command-line flag parsing
  - `fmt`: Formatted I/O
  - `log`: Logging functionality
  - `os`: Operating system functionality
  - `os/exec`: External command execution
  - `runtime`: Runtime environment information
  - `strconv`: String conversions
  - `strings`: String manipulation
  - `sync`: Synchronization primitives
  - `time`: Time-related functions

- External dependencies:
  - `github.com/pterm/pterm`: Terminal output formatting and progress indicators

## Troubleshooting

### Common Issues

1. **"Failed to read input file" error**
   - Ensure the JSON file exists and is readable
   - Verify the file path is correct

2. **"Failed to parse JSON" error**
   - Check the JSON file format for syntax errors
   - Ensure the file follows the required structure

3. **"No matching region found" message**
   - Verify country and city names match ExpressVPN's naming conventions
   - Try using only the country name without city

4. **"Failed to connect to VPN" error**
   - Ensure ExpressVPN is installed and configured correctly
   - Verify your ExpressVPN subscription is active
   - Check if `expressvpnctl` is in your PATH

5. **"Speed test failed" message**
   - Verify Speedtest CLI is installed correctly
   - Check your internet connection
   - Ensure you have permission to run speed tests

6. **Inconsistent results**
   - Try increasing the number of tests with `-r` flag
   - For gigabit connections, use the `-s` flag for sequential testing
   - Run tests at different times of day to account for network variability
