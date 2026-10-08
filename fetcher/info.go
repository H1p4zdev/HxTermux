package main

import (
	"bufio"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func collectInfo() systemInfo {
	current, _ := user.Current()
	username := envOr("USER", "unknown")
	if current != nil && current.Username != "" {
		username = current.Username
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}
	memory, memoryPercent := linuxMemory()
	return systemInfo{
		user: username, host: hostname, os: operatingSystem(), kernel: kernelRelease(),
		arch: runtime.GOARCH, shell: filepath.Base(envOr("SHELL", "unknown")),
		terminal: envOr("TERM", "unknown"), directory: envOr("PWD", "unknown"),
		uptime: linuxUptime(), memory: memory, memoryPercent: memoryPercent,
		cpu: cpuModel(), device: deviceModel(), packages: packageCount(),
		brand: getprop("ro.product.brand"), init: initSystem(), disk: diskUsage(),
	}
}

func initSystem() string {
	if runtime.GOOS == "android" || getprop("ro.build.version.release") != "" {
		return "init.rc"
	}
	if output, err := exec.Command("ps", "-p", "1", "-o", "comm=").Output(); err == nil {
		if value := strings.TrimSpace(string(output)); value != "" {
			return value
		}
	}
	return "unknown"
}

func diskUsage() string {
	path := "/"
	if runtime.GOOS == "android" {
		path = "/data"
	}
	output, err := exec.Command("df", "-h", path).Output()
	if err != nil {
		return "unknown"
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return "unknown"
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return "unknown"
	}
	return fields[2] + " / " + fields[1]
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func operatingSystem() string {
	if runtime.GOOS == "android" || runtime.GOOS == "linux" {
		if version := getprop("ro.build.version.release"); version != "" {
			return "Android " + version
		}
		file, err := os.Open("/etc/os-release")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				}
			}
		}
	}
	return runtime.GOOS
}

func getprop(name string) string {
	output, err := exec.Command("getprop", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func deviceModel() string {
	if model := getprop("ro.product.model"); model != "" {
		return model
	}
	return "unknown"
}

func cpuModel() string {
	if model := getprop("ro.soc.model"); model != "" {
		manufacturer := getprop("ro.soc.manufacturer")
		if manufacturer == "QTI" {
			manufacturer = "Qualcomm"
		}
		if manufacturer != "" {
			return manufacturer + " " + model
		}
		return model
	}
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, key := range []string{"model name", "Hardware", "Processor"} {
			if strings.HasPrefix(line, key+"\t") || strings.HasPrefix(line, key+":") {
				if _, value, ok := strings.Cut(line, ":"); ok {
					if value = strings.TrimSpace(value); value != "" {
						return value
					}
				}
			}
		}
	}
	return runtime.GOARCH
}

func packageCount() string {
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return "unknown"
	}
	output, err := exec.Command("dpkg-query", "-f=${binary:Package}\\n", "-W").Output()
	if err != nil {
		return "unknown"
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return strconv.Itoa(count) + " packages"
}

func kernelRelease() string {
	if output, err := exec.Command("uname", "-r").Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return "unknown"
}

func linuxUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "unknown"
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || seconds < 0 {
		return "unknown"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return strconv.Itoa(int(d/time.Second)) + "s"
	}
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	if days > 0 {
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
}

func linuxMemory() (string, float64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return "unknown", 0
	}
	defer file.Close()
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && (fields[0] == "MemTotal:" || fields[0] == "MemAvailable:") {
			if value, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				values[fields[0]] = value
			}
		}
	}
	total, hasTotal := values["MemTotal:"]
	available, hasAvailable := values["MemAvailable:"]
	if !hasTotal || !hasAvailable || available > total {
		return "unknown", 0
	}
	usedMiB, totalMiB := (total-available)/1024, total/1024
	return strconv.FormatUint(usedMiB, 10) + " MiB / " + strconv.FormatUint(totalMiB, 10) + " MiB", float64(total-available) / float64(total)
}
