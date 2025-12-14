package priority

import (
	"regexp"
)

type Level string

const (
	Critical    Level = "critical"
	Interesting Level = "interesting"
	Noise       Level = "noise"
)

var (
	criticalPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^/etc/passwd$`),
		regexp.MustCompile(`^/etc/shadow$`),
		regexp.MustCompile(`^/etc/sudoers`),
		regexp.MustCompile(`^/etc/ssh/`),
		regexp.MustCompile(`^/etc/cron`),
		regexp.MustCompile(`/authorized_keys$`),
		regexp.MustCompile(`^/etc/systemd/system/`),
		regexp.MustCompile(`^/usr/bin/`),
		regexp.MustCompile(`^/usr/sbin/`),
		regexp.MustCompile(`^/usr/local/bin/`),
		regexp.MustCompile(`^/usr/local/sbin/`),
	}
	interestingPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^/etc/`),
		regexp.MustCompile(`^/opt/`),
		regexp.MustCompile(`/\.bashrc$`),
		regexp.MustCompile(`/\.bash_profile$`),
		regexp.MustCompile(`/\.profile$`),
		regexp.MustCompile(`/\.zshrc$`),
	}
)

// Classify determines the priority level of a filesystem change
// Parameters:
//   - path: file path
//   - mode: file mode bits
//   - isNew: true if file was added
//   - isBulk: true if part of bulk operation
func Classify(path string, mode uint32, isNew, isBulk bool) Level {
	if isBulk {
		return Noise
	}

	// setuid/setgid bits
	if mode&0o4000 != 0 || mode&0o2000 != 0 {
		return Critical
	}

	for _, pat := range criticalPatterns {
		if pat.MatchString(path) {
			return Critical
		}
	}

	for _, pat := range interestingPatterns {
		if pat.MatchString(path) {
			return Interesting
		}
	}

	if isNew {
		return Interesting
	}

	// executable
	if mode&0o111 != 0 {
		return Interesting
	}

	return Noise
}
