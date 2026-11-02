package main

import (
	"fmt"
	"os"

	"ghosttylib-cursor-control/internal/picker"
)

func printHelp() {
	fmt.Print(`cursor-picker — Live Ghostty GPU cursor picker for ghosttylib-cursor-control

Usage:
  ./cursor-picker               Launch interactive live picker (prebuilt binary or local build)
  ./cursor-picker --build       Build cursor-picker locally from Go source (bin/cursor-picker-local)
  ./cursor-picker --build-all   Cross-compile prebuilt binaries for macOS & Linux (bin/)
  ./cursor-picker --clean-build Remove local source build and revert to prebuilt binary
  ./cursor-picker --which       Show which cursor-picker binary is currently active
  ./cursor-picker --help        Show this help message

How It Works:
  Instead of faking shaders with ASCII blocks, cursor-picker keeps Ghostty's real hardware/GPU
  cursor visible inside the preview.go stage and hot-reloads Ghostty via SIGUSR2 as you browse.
  Every shape and effect you highlight is rendered by libghostty's actual GLSL shader engine!

Interactive Controls:
  ← / → / Tab / h / l   Switch between SHAPES (9) and EFFECTS (10)
  ↑ / ↓ / j / k         Browse & live-reload real Ghostty shader on stage
  Space                 Trigger horizontal jump on stage
  w                     Trigger full-word delete (katana horizontal slash)
  v                     Trigger diagonal line jump
  p                     Pause / resume cursor movement (inspect idle cycle)
  o                     Toggle stock native cursor (off)
  a                     Save current combo as baseline without closing
  Enter                 Prompt whether to override tool colors [y/N], then save & exit
  Esc / q / Ctrl+C      Revert to previous cursor & quit
`)
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printHelp()
			return
		}
	}

	repoRoot, err := picker.ResolveRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving repo root: %v\n", err)
		os.Exit(1)
	}

	res, err := picker.RunInteractive(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch {
	case res.Saved && res.TurnedOff:
		fmt.Println("✓ Custom cursor disabled -> stock native Ghostty cursor active.")
	case res.Saved:
		colorMode := "respect tool colors"
		if res.OverrideColors {
			colorMode = "override tool colors"
		}
		fmt.Printf("✓ Active cursor -> effect: %s | shape: %s (%s) — %s\n",
			res.Effect.ID, res.Shape.ID, colorMode, res.Effect.Summary)
	case res.Reverted:
		fmt.Println("Reverted to previous cursor configuration.")
	}
}
