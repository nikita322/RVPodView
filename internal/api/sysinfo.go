package api

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HostStats represents CPU and temperature info
type HostStats struct {
	CPUUsage     float64       `json:"cpuUsage"`
	Temperatures []Temperature `json:"temperatures"`
}

// Temperature represents a temperature sensor reading
type Temperature struct {
	Label string  `json:"label"`
	Temp  float64 `json:"temp"`
}

// GetHostStats reads CPU usage and temperatures from /sys and /proc
func GetHostStats() *HostStats {
	stats := &HostStats{
		Temperatures: []Temperature{},
	}

	// Get CPU usage
	stats.CPUUsage = getCPUUsage()

	// Get temperatures from hwmon
	stats.Temperatures = getTemperatures()

	return stats
}

// getCPUUsage reads CPU usage from /proc/stat
// Returns percentage (0-100)
func getCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0
	}

	// Parse first line: cpu user nice system idle iowait irq softirq
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}

	var total, idle int64
	for i := 1; i < len(fields); i++ {
		val, _ := strconv.ParseInt(fields[i], 10, 64)
		total += val
		if i == 4 { // idle is 4th value (index 4, but fields[0] is "cpu")
			idle = val
		}
	}

	if total == 0 {
		return 0
	}

	// This is instantaneous - for better accuracy we'd need to compare two readings
	// For now return (total - idle) / total as rough estimate
	usage := float64(total-idle) / float64(total) * 100
	return usage
}

// getTemperatures reads temperatures from /sys/class/hwmon
func getTemperatures() []Temperature {
	temps := []Temperature{}

	// Scan hwmon devices
	hwmonPath := "/sys/class/hwmon"
	entries, err := os.ReadDir(hwmonPath)
	if err != nil {
		return temps
	}

	for _, entry := range entries {
		devicePath := filepath.Join(hwmonPath, entry.Name())

		// Get device name
		nameBytes, err := os.ReadFile(filepath.Join(devicePath, "name"))
		if err != nil {
			continue
		}
		deviceName := strings.TrimSpace(string(nameBytes))

		// Find temp inputs
		files, err := os.ReadDir(devicePath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasPrefix(f.Name(), "temp") || !strings.HasSuffix(f.Name(), "_input") {
				continue
			}

			// Read temperature (in millidegrees)
			tempBytes, err := os.ReadFile(filepath.Join(devicePath, f.Name()))
			if err != nil {
				continue
			}

			tempMilliC, err := strconv.ParseInt(strings.TrimSpace(string(tempBytes)), 10, 64)
			if err != nil {
				continue
			}

			tempC := float64(tempMilliC) / 1000.0

			// Try to get label
			labelFile := strings.Replace(f.Name(), "_input", "_label", 1)
			labelBytes, err := os.ReadFile(filepath.Join(devicePath, labelFile))
			label := deviceName
			if err == nil {
				label = strings.TrimSpace(string(labelBytes))
			}

			temps = append(temps, Temperature{
				Label: label,
				Temp:  tempC,
			})
		}
	}

	// Also check for NVMe temperatures
	nvmeTemps := getNVMeTemperatures()
	temps = append(temps, nvmeTemps...)

	return temps
}

// getNVMeTemperatures reads NVMe drive temperatures
func getNVMeTemperatures() []Temperature {
	temps := []Temperature{}

	// Check /sys/class/nvme
	nvmePath := "/sys/class/nvme"
	entries, err := os.ReadDir(nvmePath)
	if err != nil {
		return temps
	}

	for _, entry := range entries {
		// Try hwmon under nvme device
		hwmonPath := filepath.Join(nvmePath, entry.Name(), "hwmon")
		hwmonEntries, err := os.ReadDir(hwmonPath)
		if err != nil {
			continue
		}

		for _, hw := range hwmonEntries {
			tempFile := filepath.Join(hwmonPath, hw.Name(), "temp1_input")
			tempBytes, err := os.ReadFile(tempFile)
			if err != nil {
				continue
			}

			tempMilliC, err := strconv.ParseInt(strings.TrimSpace(string(tempBytes)), 10, 64)
			if err != nil {
				continue
			}

			temps = append(temps, Temperature{
				Label: "NVMe " + entry.Name(),
				Temp:  float64(tempMilliC) / 1000.0,
			})
		}
	}

	return temps
}
