package rsync

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var versionRe = regexp.MustCompile(`rsync\s+version\s+(\d+)\.(\d+)`)

// CheckVersion returns a warning string if the system rsync is Apple's
// openrsync (< 3.x) which broke --backup-dir in macOS Sequoia 15.4.
// Returns "" if rsync is acceptable.
func CheckVersion(binaryPath string) string {
	if binaryPath == "" {
		binaryPath = "rsync"
	}
	path, err := exec.LookPath(binaryPath)
	if err != nil {
		return fmt.Sprintf("rsync not found in PATH — install with: brew install rsync")
	}

	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	text := string(out)

	// Apple's openrsync self-identifies as "openrsync" or has version < 3.
	if strings.Contains(strings.ToLower(text), "openrsync") {
		return appleWarn(path)
	}

	m := versionRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	major, _ := strconv.Atoi(m[1])
	if major < 3 {
		return appleWarn(path)
	}
	return ""
}

func appleWarn(path string) string {
	return fmt.Sprintf(
		"rsync at %s is Apple openrsync — --backup-dir is broken in macOS 15.4+\n"+
			"  Fix: brew install rsync   (then re-run or set MEMBOX_RSYNC_PATH)",
		path,
	)
}
