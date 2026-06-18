package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func hubRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func readHubFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(hubRoot(t), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestManifestHasDualRoutesAndSingleInstance(t *testing.T) {
	text := readHubFile(t, "manifest.yml")
	checks := []string{
		"name: cf-proxy4localllm",
		"instances: 1",
		"stack: cflinuxfs4",
		"go_buildpack",
		"memory: 256M",
		"disk_quota: 512M",
		"health-check-type: http",
		"health-check-http-endpoint: /health",
		"route: cf-proxy4localllm.apps.((cf_domain))",
		"route: cf-proxy4localllm-grpc.apps.((cf_domain))",
		"protocol: http2",
		"PROXY_MODEL_ALIAS: local-ollama",
		"BRIDGE_TOKEN: ((bridge_token))",
		"GOPACKAGENAME: github.com/cf-webhook-service/cf-proxy4localllm/hub",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest.yml missing %q", want)
		}
	}
}

func TestProcfileStartsHubBinary(t *testing.T) {
	text := readHubFile(t, "Procfile")
	if !strings.Contains(text, "web: hub") {
		t.Fatalf("Procfile = %q", text)
	}
}

func TestCfignoreExcludesTestsNotVendor(t *testing.T) {
	text := readHubFile(t, ".cfignore")
	for _, want := range []string{"*_test.go", "vars.yml", ".git/"} {
		if !strings.Contains(text, want) {
			t.Fatalf(".cfignore missing %q", want)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "vendor/" || trimmed == "vendor" {
			t.Fatal(".cfignore must not exclude vendor/ — run make vendor before cf push")
		}
	}
}

func TestVarsExampleHasPlaceholders(t *testing.T) {
	text := readHubFile(t, "vars.yml.example")
	for _, want := range []string{"cf_domain:", "bridge_token:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("vars.yml.example missing %q", want)
		}
	}
}

func TestDeployScriptExists(t *testing.T) {
	path := filepath.Join(hubRoot(t), "scripts", "deploy.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("scripts/deploy.sh is not executable")
	}
}
