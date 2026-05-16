// Package config defines the Memory Box configuration schema.
// Config is loaded via Viper with priority: flags > MEMBOX_* env > config file > defaults.
package config

// Config is the top-level configuration for membox.
type Config struct {
	Drive    DriveConfig              `mapstructure:"drive"    yaml:"drive"`
	Sections map[string]SectionConfig `mapstructure:"sections" yaml:"sections"`
	Parallel int                      `mapstructure:"parallel" yaml:"parallel"`
	Prune    PruneConfig              `mapstructure:"prune"    yaml:"prune"`
	UI       UIConfig                 `mapstructure:"ui"       yaml:"ui"`
}

// DriveConfig describes where backups land — local drive or cloud storage.
type DriveConfig struct {
	// Local drive fields (backend: local or empty)
	MountPath   string `mapstructure:"mountPath"   yaml:"mountPath,omitempty"`
	BackupDir   string `mapstructure:"backupDir"   yaml:"backupDir,omitempty"`
	ArchiveDir  string `mapstructure:"archiveDir"  yaml:"archiveDir,omitempty"`
	ManifestDir string `mapstructure:"manifestDir" yaml:"manifestDir,omitempty"`
	LogDir      string `mapstructure:"logDir"      yaml:"logDir,omitempty"`

	// Cloud / rclone fields (backend: rclone)
	Backend    string `mapstructure:"backend"    yaml:"backend,omitempty"`    // "local" (default) | "rclone"
	RclonePath string `mapstructure:"rclonePath" yaml:"rclonePath,omitempty"` // e.g. "r2:my-bucket/membox"
	CacheDir   string `mapstructure:"cacheDir"   yaml:"cacheDir,omitempty"`   // local manifest cache; defaults to ~/.cache/memorybox
	CacheTTL   int    `mapstructure:"cacheTTL"   yaml:"cacheTTL,omitempty"`   // hours before manifest cache is considered stale (default 24)
}

// HooksConfig holds optional shell commands to run before/after a section backup.
type HooksConfig struct {
	Pre  string `mapstructure:"pre"  yaml:"pre,omitempty"`
	Post string `mapstructure:"post" yaml:"post,omitempty"`
}

// SectionConfig describes one rsync backup section.
type SectionConfig struct {
	Enabled        bool        `mapstructure:"enabled"        yaml:"enabled"`
	Source         string      `mapstructure:"source"         yaml:"source"` // ~ is expanded at load time
	Dest           string      `mapstructure:"dest"           yaml:"dest"`
	Excludes       []string    `mapstructure:"excludes"       yaml:"excludes"`
	Delete         bool        `mapstructure:"delete"         yaml:"delete"`
	GitignoreAware bool        `mapstructure:"gitignoreAware" yaml:"gitignoreAware,omitempty"`
	Hooks          HooksConfig `mapstructure:"hooks"          yaml:"hooks,omitempty"`
}

// PruneConfig holds defaults for the prune command.
type PruneConfig struct {
	DefaultDays int `mapstructure:"defaultDays" yaml:"defaultDays"`
	DefaultKeep int `mapstructure:"defaultKeep" yaml:"defaultKeep"`
}

// UIConfig controls terminal output behaviour.
type UIConfig struct {
	Color   bool `mapstructure:"color"   yaml:"color"`
	Verbose bool `mapstructure:"verbose" yaml:"verbose"`
	Quiet   bool `mapstructure:"quiet"   yaml:"quiet"`
}
