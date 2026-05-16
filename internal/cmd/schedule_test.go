package cmd

import (
	"strings"
	"testing"
)

func TestDarwinPlist_daily(t *testing.T) {
	opts := scheduleOpts{interval: "daily", nice: true}
	plist := darwinPlist("/usr/local/bin/membox", opts)

	checks := []string{
		"com.memorybox.backup",
		"/usr/local/bin/membox",
		"<key>Hour</key><integer>2</integer>",
		"<key>Nice</key>",
	}
	for _, c := range checks {
		if !strings.Contains(plist, c) {
			t.Errorf("plist missing %q", c)
		}
	}
}

func TestDarwinPlist_hourly(t *testing.T) {
	opts := scheduleOpts{interval: "hourly"}
	plist := darwinPlist("/usr/local/bin/membox", opts)
	if !strings.Contains(plist, "<key>Minute</key>") {
		t.Error("hourly plist missing Minute key")
	}
	if strings.Contains(plist, "<key>Hour</key>") {
		t.Error("hourly plist should not contain Hour key")
	}
}

func TestDarwinPlist_weekly(t *testing.T) {
	opts := scheduleOpts{interval: "weekly"}
	plist := darwinPlist("/usr/local/bin/membox", opts)
	if !strings.Contains(plist, "<key>Weekday</key>") {
		t.Error("weekly plist missing Weekday key")
	}
}

func TestDarwinPlist_bwlimit(t *testing.T) {
	opts := scheduleOpts{interval: "daily", bwlimit: 5000}
	plist := darwinPlist("/usr/local/bin/membox", opts)
	if !strings.Contains(plist, "--bwlimit=5000") {
		t.Error("plist missing --bwlimit flag")
	}
}

func TestDarwinPlist_noNice(t *testing.T) {
	opts := scheduleOpts{interval: "daily", nice: false}
	plist := darwinPlist("/usr/local/bin/membox", opts)
	if strings.Contains(plist, "<key>Nice</key>") {
		t.Error("Nice key present but nice=false")
	}
}

func TestLinuxService_nice(t *testing.T) {
	opts := scheduleOpts{nice: true}
	svc := linuxService("/usr/local/bin/membox", opts)
	if !strings.Contains(svc, "Nice=10") {
		t.Error("service missing Nice=10")
	}
	if !strings.Contains(svc, "IOSchedulingClass=idle") {
		t.Error("service missing IOSchedulingClass=idle")
	}
}

func TestLinuxService_bwlimit(t *testing.T) {
	opts := scheduleOpts{bwlimit: 2000}
	svc := linuxService("/usr/local/bin/membox", opts)
	if !strings.Contains(svc, "--bwlimit=2000") {
		t.Error("service missing --bwlimit flag")
	}
}

func TestLinuxTimer_intervals(t *testing.T) {
	cases := []struct {
		interval string
		want     string
	}{
		{"daily", "*-*-* 02:00:00"},
		{"hourly", "hourly"},
		{"weekly", "weekly"},
		{"", "*-*-* 02:00:00"}, // default
	}
	for _, tc := range cases {
		timer := linuxTimer(scheduleOpts{interval: tc.interval})
		if !strings.Contains(timer, tc.want) {
			t.Errorf("interval=%q: timer missing %q\n%s", tc.interval, tc.want, timer)
		}
	}
}
