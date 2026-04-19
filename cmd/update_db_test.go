package main

import "testing"

func TestNormalizePlatform(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "linux", want: "linux"},
		{in: "mac", want: "mac"},
		{in: "darwin", want: "mac"},
		{in: "windows", want: "windows"},
		{in: " WINDOWS ", want: "windows"},
	}

	for _, tc := range cases {
		got := normalizePlatform(tc.in)
		if got != tc.want {
			t.Fatalf("normalizePlatform(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPlatformDBFile(t *testing.T) {
	cases := []struct {
		platform string
		want     string
		ok       bool
	}{
		{platform: "linux", want: "commands_linux.json", ok: true},
		{platform: "mac", want: "commands_mac.json", ok: true},
		{platform: "windows", want: "commands_windows.json", ok: true},
		{platform: "darwin", want: "", ok: false},
	}

	for _, tc := range cases {
		got, err := platformDBFile(tc.platform)
		if tc.ok {
			if err != nil {
				t.Fatalf("platformDBFile(%q) returned error: %v", tc.platform, err)
			}
			if got != tc.want {
				t.Fatalf("platformDBFile(%q) = %q, want %q", tc.platform, got, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("platformDBFile(%q) expected error, got none", tc.platform)
		}
	}
}
