package shell

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestWriteWrapper_Bash(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWrapper(&buf, "bash", TemplateData{CommandName: "gwq"})
	if err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "gwq()") {
		t.Error("bash wrapper should contain gwq() function")
	}
	if !strings.Contains(output, "__GWQ_CD_SHIM=1") {
		t.Error("bash wrapper should contain __GWQ_CD_SHIM=1")
	}
	if !strings.Contains(output, "builtin cd") {
		t.Error("bash wrapper should contain builtin cd")
	}
}

func TestWriteWrapper_Zsh(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWrapper(&buf, "zsh", TemplateData{CommandName: "gwq"})
	if err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "gwq()") {
		t.Error("zsh wrapper should contain gwq() function")
	}
	if !strings.Contains(output, `[[ "$1" == "cd" ]]`) {
		t.Error("zsh wrapper should use [[ ]] syntax")
	}
	if !strings.Contains(output, "__GWQ_CD_SHIM=1") {
		t.Error("zsh wrapper should contain __GWQ_CD_SHIM=1")
	}
}

func TestWriteWrapper_Fish(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWrapper(&buf, "fish", TemplateData{CommandName: "gwq"})
	if err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "function gwq") {
		t.Error("fish wrapper should contain function gwq")
	}
	if !strings.Contains(output, "env __GWQ_CD_SHIM=1") {
		t.Error("fish wrapper should contain env __GWQ_CD_SHIM=1")
	}
}

func TestWriteWrapper_InvalidShell(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWrapper(&buf, "powershell", TemplateData{CommandName: "gwq"})
	if err == nil {
		t.Error("WriteWrapper() should return error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error should mention unsupported shell, got: %v", err)
	}
}

func TestWriteWrapper_CustomCommandName(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWrapper(&buf, "bash", TemplateData{CommandName: "mygwq"})
	if err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "mygwq()") {
		t.Error("wrapper should use custom command name")
	}
}

func TestBashSyntaxValid(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	var buf bytes.Buffer
	if err := WriteWrapper(&buf, "bash", TemplateData{CommandName: "gwq"}); err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	cmd := exec.Command("bash", "-n", "-c", buf.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("bash syntax check failed: %v\n%s", err, output)
	}
}

func TestZshSyntaxValid(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	var buf bytes.Buffer
	if err := WriteWrapper(&buf, "zsh", TemplateData{CommandName: "gwq"}); err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	cmd := exec.Command("zsh", "-n", "-c", buf.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("zsh syntax check failed: %v\n%s", err, output)
	}
}

func TestFishSyntaxValid(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish not available")
	}
	var buf bytes.Buffer
	if err := WriteWrapper(&buf, "fish", TemplateData{CommandName: "gwq"}); err != nil {
		t.Fatalf("WriteWrapper() error = %v", err)
	}
	cmd := exec.Command("fish", "--no-execute", "-c", buf.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("fish syntax check failed: %v\n%s", err, output)
	}
}
