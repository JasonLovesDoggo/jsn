package system

import (
	"io/fs"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// SystemInfo contains metadata about the system when snapshot was taken
type SystemInfo struct {
	Hostname     string        `json:"hostname"`
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	Distro       string        `json:"distro"`
	KernelVer    string        `json:"kernel_version"`
	Timestamp    time.Time     `json:"timestamp"`
	ScanRoot     string        `json:"scan_root"`
	ScanDuration time.Duration `json:"scan_duration"`
	CPUCount     int           `json:"cpu_count"`
	GoVersion    string        `json:"go_version"`
}

// FileInfo contains system-specific file information
type FileInfo struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

// GetSystemInfo gathers comprehensive system metadata
func GetSystemInfo(scanRoot string) SystemInfo {
	hostname, _ := os.Hostname()

	return SystemInfo{
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Distro:    detectDistro(),
		KernelVer: getKernelVersion(),
		Timestamp: time.Now(),
		ScanRoot:  scanRoot,
		CPUCount:  runtime.NumCPU(),
		GoVersion: runtime.Version(),
	}
}

// GetFileInfo extracts system-specific file information
func GetFileInfo(info fs.FileInfo) *FileInfo {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return &FileInfo{
			UID: int(stat.Uid),
			GID: int(stat.Gid),
		}
	}
	return nil
}

// detectDistro attempts to detect the Linux distribution
func detectDistro() string {
	// Try /etc/os-release first (standard)
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				value := strings.Split(line, "=")[1]
				return strings.Trim(value, "\"")
			}
		}
		// Fallback to NAME if PRETTY_NAME not found
		for _, line := range lines {
			if strings.HasPrefix(line, "NAME=") {
				value := strings.Split(line, "=")[1]
				return strings.Trim(value, "\"")
			}
		}
	}

	// Try other common files
	distroFiles := map[string]string{
		"/etc/redhat-release": "Red Hat",
		"/etc/debian_version": "Debian",
		"/etc/ubuntu_version": "Ubuntu",
		"/etc/centos-release": "CentOS",
		"/etc/fedora-release": "Fedora",
		"/etc/arch-release":   "Arch Linux",
		"/etc/alpine-release": "Alpine Linux",
	}

	for file, distro := range distroFiles {
		if _, err := os.Stat(file); err == nil {
			if data, err := os.ReadFile(file); err == nil {
				content := strings.TrimSpace(string(data))
				if content != "" {
					return distro + " (" + content + ")"
				}
				return distro
			}
			return distro
		}
	}

	// Fallback based on OS
	switch runtime.GOOS {
	case "linux":
		return "Linux (unknown distribution)"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	case "netbsd":
		return "NetBSD"
	default:
		return runtime.GOOS
	}
}

// getKernelVersion attempts to get the kernel version
func getKernelVersion() string {
	switch runtime.GOOS {
	case "linux":
		// Try /proc/version
		if data, err := os.ReadFile("/proc/version"); err == nil {
			parts := strings.Split(string(data), " ")
			if len(parts) >= 3 {
				return parts[2]
			}
		}

		// Try uname via /proc/sys/kernel/osrelease
		if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			return strings.TrimSpace(string(data))
		}

	case "darwin":
		// Try to get macOS version
		if data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
			content := string(data)
			if idx := strings.Index(content, "<key>ProductVersion</key>"); idx != -1 {
				start := strings.Index(content[idx:], "<string>") + idx + 8
				end := strings.Index(content[start:], "</string>") + start
				if start > idx && end > start {
					return content[start:end]
				}
			}
		}

	case "windows":
		// Windows version detection would require more complex logic
		return "Windows"
	}

	return "unknown"
}

// GetMemoryInfo returns system memory information
func GetMemoryInfo() (total, available int64) {
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// Convert from kB to bytes (rough estimate)
						if kb := parseInt64(parts[1]); kb > 0 {
							total = kb * 1024
						}
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						if kb := parseInt64(parts[1]); kb > 0 {
							available = kb * 1024
						}
					}
				}
			}
		}
	}
	return total, available
}

// GetLoadAverage returns system load average (Linux only)
func GetLoadAverage() (load1, load5, load15 float64) {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/loadavg"); err == nil {
			parts := strings.Fields(string(data))
			if len(parts) >= 3 {
				load1 = parseFloat64(parts[0])
				load5 = parseFloat64(parts[1])
				load15 = parseFloat64(parts[2])
			}
		}
	}
	return load1, load5, load15
}

// GetUptime returns system uptime (Linux only)
func GetUptime() time.Duration {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/uptime"); err == nil {
			parts := strings.Fields(string(data))
			if len(parts) >= 1 {
				if seconds := parseFloat64(parts[0]); seconds > 0 {
					return time.Duration(seconds) * time.Second
				}
			}
		}
	}
	return 0
}

// GetDiskInfo returns basic disk information for a path
func GetDiskInfo(path string) (total, free, used int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err == nil {
		total = int64(stat.Blocks) * int64(stat.Bsize)
		free = int64(stat.Bavail) * int64(stat.Bsize)
		used = total - free
	}
	return total, free, used
}

// Helper functions for parsing
func parseInt64(s string) int64 {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		} else {
			break
		}
	}
	return result
}

func parseFloat64(s string) float64 {
	var result float64
	var decimal float64 = 1
	var afterDecimal bool

	for _, c := range s {
		if c >= '0' && c <= '9' {
			if afterDecimal {
				decimal *= 0.1
				result += float64(c-'0') * decimal
			} else {
				result = result*10 + float64(c-'0')
			}
		} else if c == '.' && !afterDecimal {
			afterDecimal = true
		} else {
			break
		}
	}
	return result
}

// IsRoot checks if the current user is root/administrator
func IsRoot() bool {
	switch runtime.GOOS {
	case "windows":
		// On Windows, check if running as administrator
		// This is a simplified check
		return os.Getenv("USERNAME") == "Administrator"
	default:
		// On Unix-like systems, check if UID is 0
		return os.Getuid() == 0
	}
}

// GetCurrentUser returns information about the current user
func GetCurrentUser() (username string, uid, gid int) {
	uid = os.Getuid()
	gid = os.Getgid()

	if user := os.Getenv("USER"); user != "" {
		username = user
	} else if user := os.Getenv("USERNAME"); user != "" {
		username = user
	} else {
		username = "unknown"
	}

	return username, uid, gid
}

// GetEnvironmentInfo returns relevant environment information
func GetEnvironmentInfo() map[string]string {
	env := make(map[string]string)

	// Important environment variables for security analysis
	importantVars := []string{
		"PATH", "HOME", "USER", "USERNAME", "SHELL", "TERM",
		"LANG", "LC_ALL", "TZ", "TMPDIR", "TEMP", "TMP",
		"GOPATH", "GOROOT", "GOOS", "GOARCH",
		"SSH_CLIENT", "SSH_CONNECTION", "SSH_TTY",
		"SUDO_USER", "SUDO_UID", "SUDO_GID",
	}

	for _, varName := range importantVars {
		if value := os.Getenv(varName); value != "" {
			env[varName] = value
		}
	}

	return env
}

// GetNetworkInterfaces returns basic network interface information
func GetNetworkInterfaces() []string {
	var interfaces []string

	// On Linux, try to read /proc/net/dev
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/net/dev"); err == nil {
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if i < 2 { // Skip header lines
					continue
				}
				if strings.TrimSpace(line) == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) > 0 {
					iface := strings.TrimSuffix(parts[0], ":")
					if iface != "lo" { // Skip loopback
						interfaces = append(interfaces, iface)
					}
				}
			}
		}
	}

	return interfaces
}
