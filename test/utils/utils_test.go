package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetNonEmptyLines(t *testing.T) {
	lines := GetNonEmptyLines("alpha\n\n beta\n\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("len(GetNonEmptyLines()) = %d, want %d", got, want)
	}
	if lines[0] != "alpha" || lines[1] != " beta" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestGetProjectDirStripsE2EPath(t *testing.T) {
	baseDir := t.TempDir()
	e2eDir := filepath.Join(baseDir, "test", "e2e")
	if err := os.MkdirAll(e2eDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	withWorkingDir(t, e2eDir, func() {
		projectDir, err := GetProjectDir()
		if err != nil {
			t.Fatalf("GetProjectDir() error = %v", err)
		}
		resolvedProjectDir, err := filepath.EvalSymlinks(projectDir)
		if err != nil {
			t.Fatalf("EvalSymlinks(projectDir) error = %v", err)
		}
		resolvedBaseDir, err := filepath.EvalSymlinks(baseDir)
		if err != nil {
			t.Fatalf("EvalSymlinks(baseDir) error = %v", err)
		}
		if resolvedProjectDir != resolvedBaseDir {
			t.Fatalf("GetProjectDir() = %q, want %q", resolvedProjectDir, resolvedBaseDir)
		}
	})
}

func TestRunSuccessAndFailure(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	t.Run("success", func(t *testing.T) {
		output, err := Run(exec.Command("sh", "-c", "printf '%s' \"$GO111MODULE\""))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if output != "on" {
			t.Fatalf("Run() output = %q, want %q", output, "on")
		}
	})

	t.Run("failure", func(t *testing.T) {
		output, err := Run(exec.Command("sh", "-c", "printf 'boom' && exit 2"))
		if err == nil {
			t.Fatal("Run() error = nil, want non-nil")
		}
		if output != "boom" {
			t.Fatalf("Run() output = %q, want %q", output, "boom")
		}
		if !strings.Contains(err.Error(), "failed with error \"boom\"") {
			t.Fatalf("Run() error = %q, want wrapped output", err)
		}
	})
}

func TestIsCertManagerCRDsInstalled(t *testing.T) {
	kubectlDir := t.TempDir()
	logPath := filepath.Join(kubectlDir, "kubectl.log")
	writeExecutable(t, filepath.Join(kubectlDir, "kubectl"), kubectlStubScript)
	t.Setenv("PATH", kubectlDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COPILOT_TEST_LOG", logPath)

	t.Run("detects cert-manager CRDs", func(t *testing.T) {
		t.Setenv("KUBECTL_GET_CRDS_OUTPUT", "NAME\ncertificates.cert-manager.io\n")
		if !IsCertManagerCRDsInstalled() {
			t.Fatal("IsCertManagerCRDsInstalled() = false, want true")
		}
	})

	t.Run("returns false when CRDs are missing", func(t *testing.T) {
		t.Setenv("KUBECTL_GET_CRDS_OUTPUT", "NAME\ncustomresourcedefinitions.apiextensions.k8s.io\n")
		if IsCertManagerCRDsInstalled() {
			t.Fatal("IsCertManagerCRDsInstalled() = true, want false")
		}
	})

	t.Run("returns false when kubectl fails", func(t *testing.T) {
		t.Setenv("KUBECTL_GET_CRDS_OUTPUT", "")
		t.Setenv("KUBECTL_FAIL_CONTAINS", "get crds")
		defer t.Setenv("KUBECTL_FAIL_CONTAINS", "")
		if IsCertManagerCRDsInstalled() {
			t.Fatal("IsCertManagerCRDsInstalled() = true, want false on kubectl failure")
		}
	})
}

func TestInstallCertManager(t *testing.T) {
	kubectlDir := t.TempDir()
	logPath := filepath.Join(kubectlDir, "kubectl.log")
	writeExecutable(t, filepath.Join(kubectlDir, "kubectl"), kubectlStubScript)
	t.Setenv("PATH", kubectlDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COPILOT_TEST_LOG", logPath)

	if err := InstallCertManager(); err != nil {
		t.Fatalf("InstallCertManager() error = %v", err)
	}

	lines := readLogLines(t, logPath)
	if got, want := len(lines), 2; got != want {
		t.Fatalf("len(log lines) = %d, want %d (%v)", got, want, lines)
	}
	applyCmd := "apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml"
	if !strings.Contains(lines[0], applyCmd) {
		t.Fatalf("unexpected apply command: %q", lines[0])
	}
	waitCmd := "wait deployment.apps/cert-manager-webhook --for condition=Available" +
		" --namespace cert-manager --timeout 5m"
	if !strings.Contains(lines[1], waitCmd) {
		t.Fatalf("unexpected wait command: %q", lines[1])
	}
}

func TestInstallCertManagerReturnsKubectlErrors(t *testing.T) {
	kubectlDir := t.TempDir()
	logPath := filepath.Join(kubectlDir, "kubectl.log")
	writeExecutable(t, filepath.Join(kubectlDir, "kubectl"), kubectlStubScript)
	t.Setenv("PATH", kubectlDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COPILOT_TEST_LOG", logPath)
	t.Setenv("KUBECTL_FAIL_CONTAINS", "wait deployment.apps/cert-manager-webhook")

	if err := InstallCertManager(); err == nil {
		t.Fatal("InstallCertManager() error = nil, want non-nil")
	}
}

func TestUninstallCertManager(t *testing.T) {
	kubectlDir := t.TempDir()
	logPath := filepath.Join(kubectlDir, "kubectl.log")
	writeExecutable(t, filepath.Join(kubectlDir, "kubectl"), kubectlStubScript)
	t.Setenv("PATH", kubectlDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COPILOT_TEST_LOG", logPath)

	UninstallCertManager()

	lines := readLogLines(t, logPath)
	if got, want := len(lines), 3; got != want {
		t.Fatalf("len(log lines) = %d, want %d (%v)", got, want, lines)
	}
	deleteCmd := "delete -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml"
	if !strings.Contains(lines[0], deleteCmd) {
		t.Fatalf("unexpected uninstall command: %q", lines[0])
	}
	lease1Cmd := "delete lease cert-manager-cainjector-leader-election" +
		" -n kube-system --ignore-not-found --force --grace-period=0"
	if !strings.Contains(lines[1], lease1Cmd) {
		t.Fatalf("unexpected first lease cleanup command: %q", lines[1])
	}
	lease2Cmd := "delete lease cert-manager-controller" +
		" -n kube-system --ignore-not-found --force --grace-period=0"
	if !strings.Contains(lines[2], lease2Cmd) {
		t.Fatalf("unexpected second lease cleanup command: %q", lines[2])
	}
}

func TestLoadImageToKindClusterWithName(t *testing.T) {
	kindDir := t.TempDir()
	logPath := filepath.Join(kindDir, "kind.log")
	kindBinary := filepath.Join(kindDir, "kind-custom")
	writeExecutable(t, kindBinary, kindStubScript)
	t.Setenv("KIND", kindBinary)
	t.Setenv("KIND_CLUSTER", "cashu-test")
	t.Setenv("COPILOT_TEST_LOG", logPath)

	if err := LoadImageToKindClusterWithName("ghcr.io/example/cashu:test"); err != nil {
		t.Fatalf("LoadImageToKindClusterWithName() error = %v", err)
	}

	lines := readLogLines(t, logPath)
	if got, want := len(lines), 1; got != want {
		t.Fatalf("len(log lines) = %d, want %d (%v)", got, want, lines)
	}
	if !strings.Contains(lines[0], "load docker-image ghcr.io/example/cashu:test --name cashu-test") {
		t.Fatalf("unexpected kind command: %q", lines[0])
	}
}

func TestUncommentCode(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sample.go")
	original := "package main\n\n// fmt.Println(\"hello\")\n// fmt.Println(\"world\")\n"
	if err := os.WriteFile(file, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	target := "// fmt.Println(\"hello\")\n// fmt.Println(\"world\")"
	if err := UncommentCode(file, target, "// "); err != nil {
		t.Fatalf("UncommentCode() error = %v", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(content)
	if strings.Contains(got, "// fmt.Println") || !strings.Contains(got, "fmt.Println(\"hello\")") {
		t.Fatalf("unexpected uncommented content: %q", got)
	}
}

func TestUncommentCodeReturnsNotFoundError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := UncommentCode(file, "// missing", "// "); err == nil {
		t.Fatal("UncommentCode() error = nil, want non-nil")
	}
}

func TestWarnError(t *testing.T) {
	warnError(exec.ErrNotFound)
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	fn()
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readLogLines(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

const kubectlStubScript = `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$COPILOT_TEST_LOG"
if [ "${KUBECTL_FAIL_CONTAINS:-}" != "" ] && printf '%s' "$*" | grep -F "${KUBECTL_FAIL_CONTAINS}" >/dev/null; then
  printf '%s' "forced kubectl failure" >&2
  exit 1
fi
if [ "${1:-}" = "get" ] && [ "${2:-}" = "crds" ]; then
  printf '%s' "${KUBECTL_GET_CRDS_OUTPUT:-}"
fi
`

const kindStubScript = `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$COPILOT_TEST_LOG"
`
