package runner

import (
	"slices"
	"testing"
)

func TestTransformArgsShortcuts(t *testing.T) {
	args, model, mode := TransformArgs([]string{"--pro", "exec", "olá"}, `C:\test\catalog.json`)
	if model != "deepseek-v4-pro" || !slices.Contains(args, "deepseek-v4-pro") {
		t.Errorf("Expected --pro to set model deepseek-v4-pro, got: %v", args)
	}
	if mode != "exec" {
		t.Errorf("Expected mode exec, got: %s", mode)
	}
	if !slices.Contains(args, "--ignore-user-config") {
		t.Errorf("Expected exec subcommand to receive --ignore-user-config, got: %v", args)
	}
	if !slices.Contains(args, `tui.auto_recap=true`) {
		t.Errorf("Expected tui.auto_recap=true in overlay flags, got: %v", args)
	}
}

func TestTransformArgsModelAlias(t *testing.T) {
	args, model, _ := TransformArgs([]string{"-m", "flash", "exec", "teste"}, `C:\test\catalog.json`)
	if model != "deepseek-flash" || !slices.Contains(args, "deepseek-flash") {
		t.Errorf("Expected -m flash to resolve to deepseek-flash, got: %v", args)
	}
}
