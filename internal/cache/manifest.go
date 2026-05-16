// Package cache manages the local manifest cache for cloud backends.
// Manifests live at ~/.cache/memorybox/manifests/<encoded-remote>/<key>.manifest
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/up1512001/memorybox/internal/rclone"
)

const indexFile = "_index.json"

// Index tracks which manifests have been pulled and when.
type Index struct {
	Remote    string            `json:"remote"`
	LastSync  time.Time         `json:"lastSync"`
	Manifests map[string]string `json:"manifests"` // snapshotKey → local path
}

// ManifestCache manages local copies of remote manifests.
type ManifestCache struct {
	dir        string // e.g. ~/.cache/memorybox/manifests/r2%3Abucket%2Fmembox
	rclonePath string // e.g. r2:bucket/membox
	binaryPath string // rclone binary path
	ttl        time.Duration
}

// New returns a ManifestCache rooted at cacheDir for the given rclone remote path.
func New(cacheDir, rclonePath, rcloneBinary string, ttlHours int) (*ManifestCache, error) {
	if ttlHours <= 0 {
		ttlHours = 24
	}
	dir := filepath.Join(cacheDir, "manifests", encodeRemote(rclonePath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &ManifestCache{
		dir:        dir,
		rclonePath: rclonePath,
		binaryPath: rcloneBinary,
		ttl:        time.Duration(ttlHours) * time.Hour,
	}, nil
}

// ManifestDir returns the local cache directory for manifests.
func (c *ManifestCache) ManifestDir() string { return c.dir }

// Push copies a local manifest file to the cloud.
func (c *ManifestCache) Push(ctx context.Context, localPath, snapshotKey string) error {
	remotePath := rclone.RemoteManifestPath(c.rclonePath, snapshotKey)
	_, err := rclone.Exec(ctx, c.binaryPath,
		rclone.BuildPushManifestArgs(localPath, remotePath)...)
	if err != nil {
		return fmt.Errorf("push manifest: %w", err)
	}
	return c.updateIndex(snapshotKey, localPath)
}

// Pull fetches a specific manifest from the cloud into the local cache.
func (c *ManifestCache) Pull(ctx context.Context, snapshotKey string) (string, error) {
	localPath := filepath.Join(c.dir, snapshotKey+".manifest")
	remotePath := rclone.RemoteManifestPath(c.rclonePath, snapshotKey)
	_, err := rclone.Exec(ctx, c.binaryPath,
		rclone.BuildPullManifestArgs(remotePath, localPath)...)
	if err != nil {
		return "", fmt.Errorf("pull manifest %s: %w", snapshotKey, err)
	}
	c.updateIndex(snapshotKey, localPath) //nolint:errcheck
	return localPath, nil
}

// PullLatest fetches the N most recent manifests from the cloud.
func (c *ManifestCache) PullLatest(ctx context.Context, n int) ([]string, error) {
	remoteManifestDir := c.rclonePath + "/manifests/"
	out, err := rclone.Exec(ctx, c.binaryPath, rclone.BuildListArgs(remoteManifestDir)...)
	if err != nil {
		return nil, fmt.Errorf("list manifests: %w", err)
	}

	var files []struct {
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(out, &files); err != nil {
		return nil, fmt.Errorf("parse manifest list: %w", err)
	}

	// Sort by name descending (ISO8601 keys are lexicographically ordered).
	sorted := make([]string, len(files))
	for i, f := range files {
		sorted[i] = f.Name
	}
	sortDescending(sorted)

	if n > 0 && len(sorted) > n {
		sorted = sorted[:n]
	}

	var pulled []string
	for _, name := range sorted {
		key := strings.TrimSuffix(name, ".manifest")
		localPath, err := c.Pull(ctx, key)
		if err != nil {
			continue
		}
		pulled = append(pulled, localPath)
	}

	idx, _ := c.loadIndex()
	idx.LastSync = time.Now()
	c.saveIndex(idx) //nolint:errcheck

	return pulled, nil
}

// IsStale returns true if the cache hasn't been synced within the TTL.
func (c *ManifestCache) IsStale() bool {
	idx, err := c.loadIndex()
	if err != nil {
		return true
	}
	return time.Since(idx.LastSync) > c.ttl
}

// ── index management ──────────────────────────────────────────────────────────

func (c *ManifestCache) loadIndex() (*Index, error) {
	data, err := os.ReadFile(filepath.Join(c.dir, indexFile))
	if err != nil {
		return &Index{Remote: c.rclonePath, Manifests: make(map[string]string)}, nil
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return &Index{Remote: c.rclonePath, Manifests: make(map[string]string)}, nil
	}
	return &idx, nil
}

func (c *ManifestCache) saveIndex(idx *Index) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, indexFile), data, 0o644)
}

func (c *ManifestCache) updateIndex(key, localPath string) error {
	idx, _ := c.loadIndex()
	if idx.Manifests == nil {
		idx.Manifests = make(map[string]string)
	}
	idx.Manifests[key] = localPath
	return c.saveIndex(idx)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func encodeRemote(remote string) string {
	return strings.NewReplacer(":", "%3A", "/", "%2F").Replace(remote)
}

func sortDescending(ss []string) {
	for i := 0; i < len(ss)-1; i++ {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] > ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}
