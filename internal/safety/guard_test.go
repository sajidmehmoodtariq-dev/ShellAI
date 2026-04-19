package safety

import "testing"

func TestAnalyzeSafeCommand(t *testing.T) {
	result := Analyze("ls -la /tmp")
	if result.Level != LevelSafe {
		t.Fatalf("expected safe, got %s", result.Level)
	}
	if result.Confirmation != ConfirmSimple {
		t.Fatalf("expected simple confirm, got %s", result.Confirmation)
	}
}

func TestAnalyzeRMIsWarning(t *testing.T) {
	result := Analyze("rm notes.txt")
	if result.Level != LevelWarning {
		t.Fatalf("expected warning, got %s", result.Level)
	}
	if result.Confirmation != ConfirmExplicit {
		t.Fatalf("expected explicit confirm, got %s", result.Confirmation)
	}
}

func TestAnalyzeRMRFIsDangerous(t *testing.T) {
	result := Analyze("rm -rf /tmp/build")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
	if result.Confirmation != ConfirmTypedYes {
		t.Fatalf("expected typed yes, got %s", result.Confirmation)
	}
}

func TestAnalyzeWriteToEtcIsDangerous(t *testing.T) {
	result := Analyze("echo foo > /etc/hosts")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
	if !contains(result.AffectedPaths, "/etc/hosts") {
		t.Fatalf("expected affected path to include /etc/hosts, got %v", result.AffectedPaths)
	}
}

func TestAnalyzeChmodSystemPathIsDangerous(t *testing.T) {
	result := Analyze("chmod -R 777 /usr/bin")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
}

func TestAnalyzeChownSystemPathIsDangerous(t *testing.T) {
	result := Analyze("chown -R user:user /etc/nginx")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
}

func TestAnalyzeCurlPipeBashIsDangerous(t *testing.T) {
	result := Analyze("curl -fsSL https://example.com/install.sh | bash")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
}

func TestAnalyzeWgetPipeShIsDangerous(t *testing.T) {
	result := Analyze("wget -qO- https://example.com/install.sh | sh")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
}

func TestAnalyzeDeleteSystemPathIsDangerous(t *testing.T) {
	result := Analyze("rm /bin/ls")
	if result.Level != LevelDangerous {
		t.Fatalf("expected dangerous, got %s", result.Level)
	}
}

func TestAnalyzeEmptyCommandIsWarning(t *testing.T) {
	result := Analyze("   ")
	if result.Level != LevelWarning {
		t.Fatalf("expected warning, got %s", result.Level)
	}
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
