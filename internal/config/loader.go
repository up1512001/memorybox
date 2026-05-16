package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load merges configuration from (highest priority first):
//  1. Flags already bound to the viper instance by the caller
//  2. MEMBOX_* environment variables
//  3. Config file (~/.config/memorybox/config.yaml or --config flag)
//  4. Built-in defaults
func Load(v *viper.Viper, cfgFile string) (Config, error) {
	// Bind env vars: MEMBOX_DRIVE_MOUNTPATH → drive.mountPath
	v.SetEnvPrefix("MEMBOX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file search path.
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, _ := os.UserHomeDir()
		v.AddConfigPath(filepath.Join(home, ".config", "memorybox"))
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Apply built-in defaults so Unmarshal always has a full struct.
	defaults := Defaults()
	applyDefaults(v, defaults)

	if err := v.ReadInConfig(); err != nil {
		// Config file not found is fine — we use defaults.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("config unmarshal: %w", err)
	}

	// Expand ~ in all section source paths (may have been set via config file).
	home, _ := os.UserHomeDir()
	for name, sec := range cfg.Sections {
		if strings.HasPrefix(sec.Source, "~/") {
			sec.Source = home + sec.Source[1:]
			cfg.Sections[name] = sec
		}
	}

	return cfg, nil
}

func applyDefaults(v *viper.Viper, d Config) {
	v.SetDefault("drive.mountPath", d.Drive.MountPath)
	v.SetDefault("drive.backupDir", d.Drive.BackupDir)
	v.SetDefault("drive.archiveDir", d.Drive.ArchiveDir)
	v.SetDefault("drive.manifestDir", d.Drive.ManifestDir)
	v.SetDefault("drive.logDir", d.Drive.LogDir)
	v.SetDefault("parallel", d.Parallel)
	v.SetDefault("prune.defaultDays", d.Prune.DefaultDays)
	v.SetDefault("prune.defaultKeep", d.Prune.DefaultKeep)
	v.SetDefault("ui.color", d.UI.Color)
	v.SetDefault("ui.verbose", d.UI.Verbose)
	v.SetDefault("ui.quiet", d.UI.Quiet)
	for name, sec := range d.Sections {
		prefix := "sections." + name
		v.SetDefault(prefix+".enabled", sec.Enabled)
		v.SetDefault(prefix+".source", sec.Source)
		v.SetDefault(prefix+".dest", sec.Dest)
		v.SetDefault(prefix+".excludes", sec.Excludes)
		v.SetDefault(prefix+".delete", sec.Delete)
	}
}
