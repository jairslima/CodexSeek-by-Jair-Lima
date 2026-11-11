package runner

import (
	"slices"
	"testing"
)

func TestTransformArgsShortcuts(t *testing.T) {
	args := TransformArgs([]string{"--pro", "exec", "olá"}, `C:\test\catalog.json`)
	if !slices.Contains(args, "deepseek-v4-pro") {
		t.Errorf("Expected --pro to set model deepseek-v4-pro, got: %v", args)
	}
	if !slices.Contains(args, "--ignore-user-config") {
		t.Errorf("Expected exec subcommand to receive --ignore-user-config, got: %v", args)
	}
}

func TestTransformArgsModelAlias(t *testing.T) {
	args := TransformArgs([]string{"-m", "flash", "exec", "teste"}, `C:\test\catalog.json`)
	if !slices.Contains(args, "deepseek-flash") {
		t.Errorf("Expected -m flash to resolve to deepseek-flash, got: %v", args)
	}
}
