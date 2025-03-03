package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

var resultsFile string

type Location struct {
	Country string `json:"country"`
	City    string `json:"city"`
}

type InputData struct {
	Locations []Location `json:"locations"`
}

type Results struct {
	MachineName string    `json:"MachineName"`
	OS          string    `json:"OS"`
	WithoutVPN  string    `json:"WithoutVPN"`
	VPNStats    []VPNStat `json:"VPNStats"`
}

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

var speedTestCount = 5 // Number of parallel speed tests per VPN connection
var speedWithoutVPN string
var fileMutex sync.Mutex // Ensures safe file writes across goroutines

func main() {
	resultsFile = "results-" + time.Now().Format("20060102150405") + ".json"
	helpFlag := flag.Bool("h", false, "Display help menu")
	singleThreadedFlag := flag.Bool("s", false, "Run speed tests in series, one after another, in case of 1Gbps network")
	repeatSpeedTestFlag := flag.Int("r", 5, "Number of parallel speed tests per VPN connection")
	flag.Parse()

	if *helpFlag {
		displayHelp()
		return
	}

	if *singleThreadedFlag {
		speedTestCount = 1
	}

	if *repeatSpeedTestFlag != 0 {
		speedTestCount = *repeatSpeedTestFlag
	}

	if speedTestCount == 1 {
		fmt.Println("Running a single speed test per VPN connection")
	} else if speedTestCount > 1 {
		if *singleThreadedFlag {
			fmt.Println("Running", speedTestCount, "speed tests in series")
		} else {
			fmt.Println("Running speed tests with", speedTestCount, "parallel tests")
		}
	} else {
		log.Fatal("Number of speed tests must be at least 1")
	}

	if len(os.Args) < 1 {
		log.Fatal("Usage: expressvpnspeedtest [-s] [-r<N>] <input_file.json>")
	}

	inputFile := flag.Arg(0)
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var input InputData
	err = json.Unmarshal(data, &input)
	if err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	if *singleThreadedFlag {
		// Run speed test without VPN single threaded
		speedTest("")
	} else {
		// Run speed test without VPN multi-threaded
		runParallelSpeedTests("")
	}

	// Iterate through locations and test VPN performance
	for _, location := range input.Locations {
		region := findRegion(location)
		if region == "" {
			log.Printf("Skipping: No matching region found for %s, %s\n", location.Country, location.City)
			continue
		}

		fmt.Printf("Connecting to VPN: %s, %s...\n", location.Country, location.City)
		connectTime, err := connectToVPN(region)
		if err != nil {
			log.Printf("Failed to connect to VPN: %v\n", err)
			continue
		}

		fmt.Printf("Connected in %v\n", connectTime)

		if *singleThreadedFlag {
			// Run speed test with VPN single threaded
			speedTest(connectTime.String())
		} else {
			// Run speed test with VPN multi-threaded
			runParallelSpeedTests(connectTime.String())
		}

		// Disconnect VPN after tests
		disconnectVPN()
	}
}

// Runs speed tests in parallel and collects results
func speedTest(connectionTime string) {
	var vpnStats []VPNStat
	counter := 0

	var totalDownload, totalUpload float64
	var count int

	var spinnerText string

	for range speedTestCount {
		counter++
		if connectionTime != "" {
			spinnerText = fmt.Sprintf("Running speed test #%d through VPN...", counter)
		} else {
			spinnerText = fmt.Sprintf("Running speed test #%d without VPN...", counter)
		}
		spinner, _ := pterm.DefaultSpinner.Start(spinnerText)
		cmd := exec.Command("speedtest", "-f", "json-pretty")
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Speed test failed: %v\n", err)
			spinner.Fail("Speed test failed")

			return
		}

		var result SpeedTestResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			log.Printf("Error parsing speed test result: %v\n", err)
			spinner.Fail("Speed test failed")

			return
		}

		fmt.Println("\nLocation: ", result.Server.Country+", "+result.Server.Location)
		fmt.Println("Server: ", result.Server.Host)
		fmt.Println("Ping Latency: ", fmt.Sprintf("%.2f", result.Ping.Latency), "ms")
		fmt.Println("Download Bandwidth: ", fmt.Sprintf("%dMbps", result.Download.Bandwidth/125000))
		fmt.Println("Upload Bandwidth: ", fmt.Sprintf("%dMbps", result.Upload.Bandwidth/125000))

		if connectionTime == "" {
			speedWithoutVPN = fmt.Sprintf("%dMbps ▼  %dMbps ▲", result.Download.Bandwidth/125000, result.Upload.Bandwidth/125000)
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
		spinner.Success(fmt.Sprintf("Speed test #%d completed", counter))
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
		writeToFile(avgStat)
	}
}

// Runs speed tests in parallel and collects results
func runParallelSpeedTests(connectionTime string) {
	var wg sync.WaitGroup
	resultsChan := make(chan VPNStat, speedTestCount)

	var totalDownload, totalUpload float64
	var count int

	var spinnerText string
	if connectionTime != "" {
		spinnerText = "Running speed tests through VPN..."
	} else {
		spinnerText = "Running speed tests without VPN..."
	}

	for range speedTestCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			spinner, _ := pterm.DefaultSpinner.Start(spinnerText)
			cmd := exec.Command("speedtest", "-f", "json-pretty")
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("Speed test failed: %v\n", err)
				spinner.Fail("Speed test failed")

				return
			}

			var result SpeedTestResult
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				log.Printf("Error parsing speed test result: %v\n", err)
				spinner.Fail("Speed test failed")

				return
			}

			fmt.Println("\nLocation: ", result.Server.Country+", "+result.Server.Location)
			fmt.Println("Server: ", result.Server.Host)
			fmt.Println("Ping Latency: ", fmt.Sprintf("%.2f", result.Ping.Latency), "ms")
			fmt.Println("Download Bandwidth: ", fmt.Sprintf("%dMbps", result.Download.Bandwidth/125000))
			fmt.Println("Upload Bandwidth: ", fmt.Sprintf("%dMbps", result.Upload.Bandwidth/125000))

			if connectionTime == "" {
				speedWithoutVPN = fmt.Sprintf("%dMbps ▼  %dMbps ▲", result.Download.Bandwidth/125000, result.Upload.Bandwidth/125000)
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
			spinner.Success("Speed tests completed")
		}()
	}

	wg.Wait()
	close(resultsChan)

	// Compute the average speed
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
		writeToFile(avgStat)
	}
}

func GetOSVersion() string {
	switch runtime.GOOS {
	case "linux":
		out, _ := exec.Command("lsb_release", "-d").Output()
		return strings.TrimSpace(strings.Split(string(out), ":")[1])
	case "darwin":
		out, _ := exec.Command("sw_vers", "-productVersion").Output()
		return "macOS " + strings.TrimSpace(string(out))
	case "windows":
		out, _ := exec.Command("cmd", "/C", "ver").Output()
		return strings.TrimSpace(string(out))
	default:
		return "Unknown OS"
	}
}

// Finds the correct VPN region for a given location
func findRegion(location Location) string {
	commandOutput, err := getCommandOutput()
	if err != nil {
		fmt.Println("Error executing command:", err)
		return ""
	}

	formattedLocation := fmt.Sprintf("%s-%s", strings.ToLower(location.Country), strings.ToLower(location.City))
	countryOnly := strings.ToLower(location.Country)

	for _, loc := range commandOutput {
		if loc == formattedLocation {
			return formattedLocation
		} else if loc == countryOnly {
			return countryOnly
		}
	}

	return ""
}

// Writes speed test results to a file
func writeToFile(newStats VPNStat) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	data, err := loadFromFile(resultsFile)
	if err != nil {
		fmt.Println("Error loading JSON file:", err)
		return
	}

	if data.MachineName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			fmt.Println("Error getting hostname:", err)
			return
		}

		osName := runtime.GOOS
		osVersion := GetOSVersion()

		data = Results{
			MachineName: hostname,
			OS:          osName + ": " + osVersion,
			WithoutVPN:  speedWithoutVPN,
			VPNStats:    []VPNStat{},
		}
	}

	data.VPNStats = append(data.VPNStats, newStats)

	if err := saveToFile(data, resultsFile); err != nil {
		fmt.Println("Error saving JSON file:", err)
	}
}

// Load results from file
func loadFromFile(fileName string) (Results, error) {
	var data Results
	file, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return Results{}, nil
		}
		return data, err
	}
	err = json.Unmarshal(file, &data)
	return data, err
}

// Saves results to a JSON file
func saveToFile(data Results, fileName string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fileName, jsonData, 0644)
}

// Executes the VPN command to fetch available regions
func getCommandOutput() ([]string, error) {
	cmd := exec.Command("expressvpnctl", "get", "regions")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	return lines, nil
}

// Connects to a VPN region
func connectToVPN(region string) (time.Duration, error) {
	start := time.Now()
	cmd := exec.Command("expressvpnctl", "connect", region)
	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	waitForConnection()
	return time.Since(start).Round(time.Millisecond), nil
}

// Disconnects the VPN
func disconnectVPN() error {
	cmd := exec.Command("expressvpnctl", "disconnect")
	return cmd.Run()
}

// Ensures VPN is connected before running tests
func waitForConnection() {
	for {
		cmd := exec.Command("expressvpnctl", "get", "connectionstate")
		var out bytes.Buffer
		cmd.Stdout = &out

		err := cmd.Run()
		if err == nil && strings.TrimSpace(out.String()) == "Connected" {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func displayHelp() {
	fmt.Println("Usage: expressvpnspeedtest <input_file.json>")
	fmt.Println("Options:")
	fmt.Println("  -h     Show this help message and exit")
	fmt.Println("  -s     Run speed tests in series, one after another, in case of 1Gbps network")
	fmt.Println("  -r N   Set the number of parallel speed tests (default: 5)")
	fmt.Println("Example:")
	fmt.Println("  expressvpnspeedtest [--repeatSpeedTest 10] locations.json")
	fmt.Println("Input file format example:")
	fmt.Println(`  {
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
	  }
    ]
}`)
}
