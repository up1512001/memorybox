package rsync

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reTotalTransferred = regexp.MustCompile(`Total transferred file size:\s+([\d,]+)`)
	reTotalFileSize    = regexp.MustCompile(`Total file size:\s+([\d,]+)`)
	reFilesTransferred = regexp.MustCompile(`Number of regular files transferred:\s+([\d,]+)`)
)

// IsChanged reports whether an itemize-changes line represents a file update.
// Lines starting with ">f" mean a file was transferred.
func IsChanged(line string) bool {
	return len(line) > 1 && line[0] == '>' && line[1] == 'f'
}

// IsDeleted reports whether the line represents a deleted file being archived.
func IsDeleted(line string) bool {
	return strings.HasPrefix(line, "*deleting")
}

// Path extracts the file path from an itemize-changes line.
// Itemize lines look like: ">f+++++++ path/to/file" — path starts at index 10.
func Path(line string) string {
	if len(line) <= 10 {
		return ""
	}
	return strings.TrimSpace(line[10:])
}

// parseStats extracts aggregate numbers from rsync --stats output.
func parseStats(output string) *Stats {
	s := &Stats{}
	if m := reFilesTransferred.FindStringSubmatch(output); len(m) == 2 {
		s.FilesTransferred = parseCommaInt(m[1])
	}
	if m := reTotalTransferred.FindStringSubmatch(output); len(m) == 2 {
		s.TotalTransferred = parseCommaInt(m[1])
	}
	if m := reTotalFileSize.FindStringSubmatch(output); len(m) == 2 {
		s.TotalFileSize = parseCommaInt(m[1])
	}
	return s
}

func parseCommaInt(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
