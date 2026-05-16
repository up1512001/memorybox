package config

import "os"

// DefaultDrivePath is the mount point of the backup SSD.
const DefaultDrivePath = "/Volumes/X10 Pro"

// DefaultSections returns the built-in section registry with ~ expanded.
func DefaultSections() map[string]SectionConfig {
	home := os.Getenv("HOME")
	expand := func(p string) string {
		if len(p) >= 2 && p[:2] == "~/" {
			return home + p[1:]
		}
		return p
	}

	devExcludes := []string{
		"node_modules", "vendor", "dist", "build",
		".next", ".nuxt", "*.sql", "mysql", "logs", ".git",
	}

	return map[string]SectionConfig{
		"photos": {
			Enabled: true,
			Source:  expand("~/Pictures"),
			Dest:    "Pictures",
			Delete:  true,
		},
		"movies": {
			Enabled: true,
			Source:  expand("~/Movies"),
			Dest:    "Movies",
			Delete:  true,
		},
		"docs": {
			Enabled:  true,
			Source:   expand("~/Documents"),
			Dest:     "Documents",
			Delete:   true,
		},
		"desktop": {
			Enabled: true,
			Source:  expand("~/Desktop"),
			Dest:    "Desktop",
			Delete:  true,
		},
		"downloads": {
			Enabled: true,
			Source:  expand("~/Downloads"),
			Dest:    "Downloads",
			Delete:  true,
		},
		"icloud": {
			Enabled: true,
			Source:  home + "/Library/Mobile Documents/com~apple~CloudDocs",
			Dest:    "iCloud Drive",
			Delete:  true,
		},
		"dev": {
			Enabled:  true,
			Source:   expand("~/Developer"),
			Dest:     "Developer",
			Excludes: devExcludes,
			Delete:   true,
		},
		"localsites": {
			Enabled:  true,
			Source:   expand("~/Local Sites"),
			Dest:     "Local Sites",
			Excludes: devExcludes,
			Delete:   true,
		},
		"config": {
			Enabled: true,
			Source:  "",  // special: assembled from dotfiles at runtime
			Dest:    "dotfiles",
			Delete:  false,
		},
	}
}

// Defaults returns the base Config with all defaults applied.
func Defaults() Config {
	mount := DefaultDrivePath
	return Config{
		Drive: DriveConfig{
			MountPath:   mount,
			BackupDir:   mount + "/backup-current",
			ArchiveDir:  mount + "/backup-archive",
			ManifestDir: mount + "/backup-manifests",
			LogDir:      mount + "/backup-logs",
		},
		Sections: DefaultSections(),
		Parallel: 2,
		Prune: PruneConfig{
			DefaultDays: 90,
			DefaultKeep: 8,
		},
		UI: UIConfig{
			Color:   true,
			Verbose: false,
			Quiet:   false,
		},
	}
}
