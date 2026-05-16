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

// DriveConfig describes where backups land on the external drive.
type DriveConfig struct {
	MountPath   string `mapstructure:"mountPath"   yaml:"mountPath"`
	BackupDir   string `mapstructure:"backupDir"   yaml:"backupDir"`
	ArchiveDir  string `mapstructure:"archiveDir"  yaml:"archiveDir"`
	ManifestDir string `mapstructure:"manifestDir" yaml:"manifestDir"`
	LogDir      string `mapstructure:"logDir"      yaml:"logDir"`
}

// SectionConfig describes one rsync backup section.
type SectionConfig struct {
	Enabled  bool     `mapstructure:"enabled"  yaml:"enabled"`
	Source   string   `mapstructure:"source"   yaml:"source"`   // ~ is expanded at load time
	Dest     string   `mapstructure:"dest"     yaml:"dest"`
	Excludes []string `mapstructure:"excludes" yaml:"excludes"`
	Delete   bool     `mapstructure:"delete"   yaml:"delete"`
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
