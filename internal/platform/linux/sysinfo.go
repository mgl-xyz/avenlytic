//go:build linux

package linux

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

const maxHardwareSensors = 128

func cpuIdentity(procRoot string) (string, string, float64) {
	file, err := os.Open(filepath.Join(procRoot, "cpuinfo"))
	if err != nil {
		return "", "", 0
	}
	defer file.Close()

	var modelName, vendor string
	var currentFrequency float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "model name", "hardware":
			if modelName == "" && value != "" {
				modelName = value
			}
		case "processor":
			if modelName == "" && value != "" {
				if _, err := strconv.Atoi(value); err != nil {
					modelName = value
				}
			}
		case "vendor_id", "cpu implementer":
			if vendor == "" && value != "" {
				vendor = value
			}
		case "cpu mhz":
			if currentFrequency == 0 {
				if frequency, err := strconv.ParseFloat(value, 64); err == nil && frequency > 0 {
					currentFrequency = frequency
				}
			}
		}
	}
	return modelName, vendor, currentFrequency
}

func cpuFrequencies(sysRoot string) (current, minimum, maximum float64) {
	cpuRoot := filepath.Join(sysRoot, "devices/system/cpu")
	entries, err := os.ReadDir(cpuRoot)
	if err != nil {
		return 0, 0, 0
	}
	var currentTotal float64
	var currentCount uint64
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "cpu") {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "cpu")); err != nil {
			continue
		}
		frequencyRoot := filepath.Join(cpuRoot, entry.Name(), "cpufreq")
		if value, ok := readPositiveUint(filepath.Join(frequencyRoot, "scaling_cur_freq")); ok {
			currentTotal += float64(value)
			currentCount++
		}
		if value, ok := readPositiveUint(filepath.Join(frequencyRoot, "cpuinfo_min_freq")); ok && (minimum == 0 || float64(value) < minimum) {
			minimum = float64(value)
		}
		if value, ok := readPositiveUint(filepath.Join(frequencyRoot, "cpuinfo_max_freq")); ok && float64(value) > maximum {
			maximum = float64(value)
		}
	}
	if currentCount > 0 {
		current = currentTotal / float64(currentCount)
	}
	return current / 1000, minimum / 1000, maximum / 1000
}

func collectHardware(sysRoot string) (*model.Hardware, error) {
	info, err := os.Stat(sysRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect sys root %q: %w", sysRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("sys root %q is not a directory", sysRoot)
	}

	hwmonRoot := filepath.Join(sysRoot, "class/hwmon")
	entries, err := os.ReadDir(hwmonRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read hwmon: %w", err)
		}
		entries = nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	hardware := &model.Hardware{}
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(hwmonRoot, entry.Name()))
		if err != nil || !info.IsDir() {
			continue
		}
		sensorRoot := filepath.Join(hwmonRoot, entry.Name())
		driver, _ := os.ReadFile(filepath.Join(sensorRoot, "name"))
		driverName := strings.TrimSpace(string(driver))
		files, err := os.ReadDir(sensorRoot)
		if err != nil {
			continue
		}
		for _, file := range files {
			if len(hardware.Sensors) >= maxHardwareSensors {
				return hardware, nil
			}
			parts := strings.Split(file.Name(), "_")
			if len(parts) != 2 || (parts[1] != "input" && parts[1] != "label") {
				continue
			}
			kind := ""
			switch {
			case strings.HasPrefix(parts[0], "temp") && parts[1] == "input":
				kind = "temperature"
			case strings.HasPrefix(parts[0], "fan") && parts[1] == "input":
				kind = "fan"
			default:
				continue
			}
			raw, err := os.ReadFile(filepath.Join(sensorRoot, file.Name()))
			if err != nil {
				continue
			}
			value, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
			if err != nil {
				continue
			}
			name := parts[0]
			if label, err := os.ReadFile(filepath.Join(sensorRoot, parts[0]+"_label")); err == nil {
				if strings.TrimSpace(string(label)) != "" {
					name = strings.TrimSpace(string(label))
				}
			}
			if driverName != "" {
				name = driverName + " " + name
			}
			sensor := model.HardwareSensor{Name: name, Kind: kind, Value: value, Unit: "rpm"}
			if kind == "temperature" {
				sensor.Value = value / 1000
				sensor.Unit = "C"
				if critical, ok := readFloat(filepath.Join(sensorRoot, parts[0]+"_crit")); ok {
					critical /= 1000
					sensor.Critical = &critical
				}
			}
			hardware.Sensors = append(hardware.Sensors, sensor)
		}
	}
	if len(hardware.Sensors) == 0 {
		thermalSensors, err := collectThermalSensors(filepath.Join(sysRoot, "class/thermal"))
		if err != nil {
			return nil, err
		}
		hardware.Sensors = append(hardware.Sensors, thermalSensors...)
	}
	if len(hardware.Sensors) == 0 {
		return nil, nil
	}
	return hardware, nil
}

func collectThermalSensors(root string) ([]model.HardwareSensor, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read thermal zones: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var sensors []model.HardwareSensor
	for _, entry := range entries {
		zone := filepath.Join(root, entry.Name())
		info, err := os.Stat(zone)
		if err != nil || !info.IsDir() {
			continue
		}
		value, ok := readFloat(filepath.Join(zone, "temp"))
		if !ok {
			continue
		}
		name := entry.Name()
		if zoneType, err := os.ReadFile(filepath.Join(zone, "type")); err == nil && strings.TrimSpace(string(zoneType)) != "" {
			name = strings.TrimSpace(string(zoneType))
		}
		sensor := model.HardwareSensor{Name: name, Kind: "temperature", Value: value / 1000, Unit: "C"}
		if critical := thermalCriticalTemperature(zone); critical != nil {
			value := *critical / 1000
			sensor.Critical = &value
		}
		sensors = append(sensors, sensor)
	}
	return sensors, nil
}

func thermalCriticalTemperature(zone string) *float64 {
	entries, err := os.ReadDir(zone)
	if err != nil {
		return nil
	}
	var critical *float64
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "trip_point_") || !strings.HasSuffix(entry.Name(), "_temp") {
			continue
		}
		index := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "trip_point_"), "_temp")
		kind, err := os.ReadFile(filepath.Join(zone, "trip_point_"+index+"_type"))
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(string(kind))) {
		case "critical", "hot":
		default:
			continue
		}
		value, ok := readFloat(filepath.Join(zone, entry.Name()))
		if ok && (critical == nil || value < *critical) {
			critical = &value
		}
	}
	return critical
}

func readPositiveUint(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return value, err == nil && value > 0
}

func readFloat(path string) (float64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	return value, err == nil
}
