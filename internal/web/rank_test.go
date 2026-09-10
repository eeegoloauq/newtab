package web

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed rank_test.js
var rankTestJS string

// The ranking is JavaScript, so testing it means running it. The filter
// it replaced shipped for months answering "lo" with the wrong row, and
// no Go test could have caught that: the rule lived in the browser.
// node is in the CI image because actions/checkout needs one.
func TestRankJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node in PATH")
	}
	file := filepath.Join(t.TempDir(), "rank_test.js")
	if err := os.WriteFile(file, []byte(rankJS+rankTestJS), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, file).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("ranking:\n%s", out)
	}
}
