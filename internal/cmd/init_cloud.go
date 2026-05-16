package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// cloudProvider describes one supported cloud provider and the rclone
// config fields needed to set it up.
type cloudProvider struct {
	name        string
	rcloneType  string
	description string
	costNote    string
	setup       func(in *bufio.Reader, w io.Writer) (remoteName, rclonePath string, err error)
}

var cloudProviders = []cloudProvider{
	{
		name:        "Cloudflare R2",
		description: "S3-compatible object storage",
		costNote:    "$0 egress · $0.015/GB storage",
		setup:       setupR2,
	},
	{
		name:        "AWS S3",
		description: "Amazon Simple Storage Service",
		costNote:    "$0.09/GB egress · $0.023/GB storage",
		setup:       setupS3,
	},
	{
		name:        "Backblaze B2",
		description: "Low-cost object storage",
		costNote:    "3× free egress · $0.006/GB storage",
		setup:       setupB2,
	},
	{
		name:        "Other (manual rclone path)",
		description: "Any rclone-compatible remote you've already configured",
		costNote:    "",
		setup:       setupManual,
	},
}

// wizardCloudSetup replaces wizardPickRcloneRemote.
// It checks rclone is installed, picks a provider, collects credentials,
// creates the rclone remote, and returns the rclone path for membox config.
func wizardCloudSetup(in *bufio.Reader, w io.Writer) (rclonePath string, err error) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Cloud storage setup")
	fmt.Fprintln(w, strings.Repeat("─", 52))
	fmt.Fprintln(w)

	// Ensure rclone is installed.
	if _, err := exec.LookPath("rclone"); err != nil {
		fmt.Fprintln(w, "  rclone is not installed.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Install options:")
		fmt.Fprintln(w, "    macOS:  brew install rclone")
		fmt.Fprintln(w, "    Linux:  sudo apt install rclone  (or snap install rclone)")
		fmt.Fprintln(w, "    All:    https://rclone.org/install/")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Install rclone then run `membox init` again.")
		return "", fmt.Errorf("rclone not found — see install instructions above")
	}

	// Pick provider.
	fmt.Fprintln(w, "  Choose cloud provider:")
	fmt.Fprintln(w)
	for i, p := range cloudProviders {
		if p.costNote != "" {
			fmt.Fprintf(w, "  [%d] %-20s  %s  (%s)\n", i+1, p.name, p.description, p.costNote)
		} else {
			fmt.Fprintf(w, "  [%d] %-20s  %s\n", i+1, p.name, p.description)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Pick provider [1]: ")

	line := readLine(in)
	if line == "" {
		line = "1"
	}
	idx := 0
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(cloudProviders) {
		return "", fmt.Errorf("invalid selection %q", line)
	}
	provider := cloudProviders[idx-1]

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Setting up %s…\n", provider.name)
	fmt.Fprintln(w)

	remoteName, path, err := provider.setup(in, w)
	if err != nil {
		return "", err
	}

	// Verify the remote is reachable.
	fmt.Fprintf(w, "\n  Verifying connection to %s… ", path)
	out, testErr := exec.Command("rclone", "lsd", path).CombinedOutput()
	if testErr != nil {
		// lsd fails on empty buckets — try ls instead.
		out, testErr = exec.Command("rclone", "ls", "--max-depth", "1", path).CombinedOutput()
	}
	if testErr != nil {
		fmt.Fprintln(w, "✗")
		fmt.Fprintf(w, "  Could not access %s\n", path)
		fmt.Fprintf(w, "  %s\n", strings.TrimSpace(string(out)))
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  To troubleshoot: rclone lsd %s\n", path)
		fmt.Fprintf(w, "  To reconfigure:  rclone config reconnect %s:\n", remoteName)
		return "", fmt.Errorf("cloud connection failed — fix rclone config and re-run `membox init`")
	}
	fmt.Fprintln(w, "✓")

	return path, nil
}

// ── provider setup functions ──────────────────────────────────────────────────

func setupR2(in *bufio.Reader, w io.Writer) (remoteName, rclonePath string, err error) {
	fmt.Fprintln(w, "  Cloudflare R2 credentials")
	fmt.Fprintln(w, "  Get these from: dash.cloudflare.com → R2 → Manage API tokens")
	fmt.Fprintln(w)

	accountID := prompt(in, w, "  Account ID          : ")
	keyID := prompt(in, w, "  Access Key ID       : ")
	secret := promptSecret(w, "  Secret Access Key   : ")
	bucket := prompt(in, w, "  Bucket name         : ")

	if accountID == "" || keyID == "" || secret == "" || bucket == "" {
		return "", "", fmt.Errorf("all fields are required")
	}

	remoteName = "membox-r2"
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	if err := rcloneConfigCreate(remoteName, "s3",
		"provider", "Cloudflare",
		"access_key_id", keyID,
		"secret_access_key", secret,
		"endpoint", endpoint,
		"acl", "private",
	); err != nil {
		return "", "", err
	}

	return remoteName, remoteName + ":" + bucket + "/membox", nil
}

func setupS3(in *bufio.Reader, w io.Writer) (remoteName, rclonePath string, err error) {
	fmt.Fprintln(w, "  AWS S3 credentials")
	fmt.Fprintln(w, "  Get these from: console.aws.amazon.com → IAM → Access Keys")
	fmt.Fprintln(w, "  Or leave both blank to use IAM role / instance profile (EC2/ECS).")
	fmt.Fprintln(w)

	keyID := prompt(in, w, "  Access Key ID (blank for IAM): ")
	var secret string
	if keyID != "" {
		secret = promptSecret(w, "  Secret Access Key           : ")
	}
	region := prompt(in, w, "  Region (e.g. us-east-1)     : ")
	bucket := prompt(in, w, "  Bucket name                 : ")

	if region == "" || bucket == "" {
		return "", "", fmt.Errorf("region and bucket are required")
	}

	remoteName = "membox-s3"
	args := []string{"s3",
		"provider", "AWS",
		"region", region,
	}
	if keyID != "" {
		args = append(args, "access_key_id", keyID, "secret_access_key", secret)
	} else {
		args = append(args, "env_auth", "true")
	}

	if err := rcloneConfigCreate(remoteName, args[0], args[1:]...); err != nil {
		return "", "", err
	}

	return remoteName, remoteName + ":" + bucket + "/membox", nil
}

func setupB2(in *bufio.Reader, w io.Writer) (remoteName, rclonePath string, err error) {
	fmt.Fprintln(w, "  Backblaze B2 credentials")
	fmt.Fprintln(w, "  Get these from: backblaze.com → App Keys → Add a New Application Key")
	fmt.Fprintln(w)

	keyID := prompt(in, w, "  Application Key ID : ")
	appKey := promptSecret(w, "  Application Key    : ")
	bucket := prompt(in, w, "  Bucket name        : ")

	if keyID == "" || appKey == "" || bucket == "" {
		return "", "", fmt.Errorf("all fields are required")
	}

	remoteName = "membox-b2"
	if err := rcloneConfigCreate(remoteName, "b2",
		"account", keyID,
		"key", appKey,
	); err != nil {
		return "", "", err
	}

	return remoteName, remoteName + ":" + bucket + "/membox", nil
}

func setupManual(in *bufio.Reader, w io.Writer) (remoteName, rclonePath string, err error) {
	fmt.Fprintln(w, "  Available rclone remotes:")
	remotes := listRcloneRemotes()
	if len(remotes) == 0 {
		fmt.Fprintln(w, "  (none — run `rclone config` to create one)")
	} else {
		for _, r := range remotes {
			fmt.Fprintf(w, "    • %s\n", r)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Example paths: r2:bucket/membox · s3:bucket/folder · b2:bucket")
	fmt.Fprintln(w)
	path := prompt(in, w, "  rclone remote path: ")
	if path == "" {
		return "", "", fmt.Errorf("path cannot be empty")
	}
	// Extract remote name (everything before the first colon).
	remote := strings.SplitN(path, ":", 2)[0]
	return remote, path, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func listRcloneRemotes() []string {
	out, err := exec.Command("rclone", "listremotes").Output()
	if err != nil {
		return nil
	}
	var remotes []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes
}

// rcloneConfigCreate runs: rclone config create <name> <type> [key value ...]
// and overwrites any existing remote with the same name.
func rcloneConfigCreate(name, rcloneType string, kvPairs ...string) error {
	args := []string{"config", "create", name, rcloneType}
	args = append(args, kvPairs...)
	out, err := exec.Command("rclone", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create rclone remote: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func prompt(in *bufio.Reader, w io.Writer, label string) string {
	fmt.Fprint(w, label)
	return readLine(in)
}

// promptSecret reads a line without terminal echo (password-style).
// Falls back to plain readline if stdin is not a terminal (e.g. tests/pipes).
func promptSecret(w io.Writer, label string) string {
	fmt.Fprint(w, label)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(w) // newline after hidden input
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	// Fallback for non-TTY (pipes, tests).
	in := bufio.NewReader(os.Stdin)
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}
