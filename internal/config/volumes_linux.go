//go:build linux

package config

// AvailableVolumes returns nothing on Linux — user must specify a path manually.
func AvailableVolumes() []string {
	return nil
}
