package common

import (
	"bufio"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"tmeet/internal/config"
)

// SystemInfo holds system information.
type SystemInfo struct {
	MachineID string // Machine ID
	OS        string // 操作系统（含版本号，如 macOS-15.3）
	Agent     string // AI-Agent
	Model     string // LLM model
}

// machineIDFile is the filename for locally caching the machine ID.
const machineIDFile = ".machine_id"

// GetSystemInfo retrieves system information.
// It first tries to read the machineID from the local cache file;
// if not found, it generates a new one and writes it to the file (only the first writer succeeds in concurrent scenarios).
func GetSystemInfo() *SystemInfo {
	cacheFile := filepath.Join(config.GetConfigDir(), machineIDFile)

	agent := os.Getenv("TMEET_AGENT")
	model := os.Getenv("TMEER_MODEL")

	// 1. Try to read from local file first.
	if id, err := readMachineIDFromFile(cacheFile); err == nil && id != "" {
		return &SystemInfo{MachineID: id, OS: getOSVersion(), Agent: agent, Model: model}
	}

	// 2. No local cache, generate a new machineID.
	id := generateRandomMachineID()

	// 3. Write to local file; O_EXCL ensures only the first operation succeeds in concurrent scenarios.
	_ = writeMachineIDToFile(cacheFile, id)

	return &SystemInfo{MachineID: id, OS: getOSVersion(), Agent: agent, Model: model}
}

// readMachineIDFromFile reads the machineID from a local file.
func readMachineIDFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// writeMachineIDToFile writes the machineID to a local file.
// Uses O_CREATE|O_EXCL flags to ensure only the first call succeeds in concurrent scenarios.
func writeMachineIDToFile(path string, id string) error {
	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// O_EXCL: returns an error if the file already exists, ensuring only the first writer succeeds in concurrent scenarios.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		// 文件已存在（其他并发已写入）或其他错误，均忽略
		return err
	}
	defer f.Close()
	_, err = f.WriteString(id)
	return err
}

// getOSVersion returns the operating system type and version number.
func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: get version via sw_vers, e.g. macOS-15.3.1
		out, err := exec.Command("sw_vers", "-productVersion").Output()
		if err == nil {
			return "macOS-" + strings.TrimSpace(string(out))
		}
		return "macOS"
	case "windows":
		// Windows: get full version via ver command, e.g. Windows-10.0.19045
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			// Output format: Microsoft Windows [Version 10.0.22000.xxx]
			if idx := strings.Index(s, "Version "); idx != -1 {
				ver := strings.TrimRight(s[idx+8:], "]")
				return "Windows-" + strings.TrimSpace(ver)
			}
		}
		return "Windows"
	case "linux":
		// Linux: prefer reading PRETTY_NAME from /etc/os-release, e.g. Linux-Ubuntu 22.04.3 LTS
		if name := readOSReleasePrettyName(); name != "" {
			return "Linux-" + name
		}
		// 降级：使用内核版本
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			return "Linux-" + strings.TrimSpace(string(out))
		}
		return "Linux"
	default:
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			return runtime.GOOS + "-" + strings.TrimSpace(string(out))
		}
		return runtime.GOOS
	}
}

// generateRandomMachineID generates a random 48-character machine ID
// by randomly picking characters from the alphanumeric charset.
func generateRandomMachineID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 48
	result := make([]byte, length)
	// Read 1 byte of random data at a time, mapped to the charset index.
	buf := make([]byte, 1)
	for i := 0; i < length; i++ {
		for {
			if _, err := rand.Read(buf); err != nil {
				result[i] = charset[i%len(charset)]
				break
			}
			// Rejection sampling to ensure uniform distribution.
			if int(buf[0]) < 256-(256%len(charset)) {
				result[i] = charset[int(buf[0])%len(charset)]
				break
			}
		}
	}
	return string(result)
}

// readOSReleasePrettyName reads the PRETTY_NAME field from /etc/os-release.
func readOSReleasePrettyName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	return parsePrettyName(string(data))
}

// parsePrettyName parses the PRETTY_NAME field value from os-release file content.
func parsePrettyName(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			// Strip surrounding quotes.
			val = strings.Trim(val, `"`)
			return val
		}
	}
	return ""
}
