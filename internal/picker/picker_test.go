package picker

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var stripAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string {
	return stripAnsiRe.ReplaceAllString(s, "")
}

func TestTUIFrameExactColumnAlignmentAndStageCoordinates(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "shaders"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "configs"), 0o755)

	app := NewApp(tmp)
	app.savedState.OverrideColors = true
	app.savedState.OverrideShapes = true
	for _, confirm := range []confirmPromptMode{confirmNone, confirmSaveExit, confirmSaveExitShapes} {
		app.confirmMode = confirm
		for _, size := range [][2]int{{86, 24}, {100, 26}, {116, 30}, {132, 40}} {
			lines, origin := app.RenderFrame(size[0], size[1])
			if len(lines) == 0 {
				t.Fatalf("empty frame at %v", size)
			}
			wantW := VisibleLen(lines[0])
			for idx, ln := range lines {
				gotW := VisibleLen(ln)
				if gotW != wantW {
					t.Fatalf("confirm=%v size %v line %d visible width mismatch: got %d, want %d\nline=%q", confirm, size, idx, gotW, wantW, ln)
				}
			}

			// Verify 1-based StageOrigin lands on the EXACT characters inside preview.go!
			line0Runes := []rune(stripANSI(lines[origin.TopRow-1+0]))
			if got := line0Runes[origin.LeftCol-1+1]; got != '1' {
				t.Fatalf("size %v expected line number '1' at stage (0,1), got %q", size, got)
			}
			line3Runes := []rune(stripANSI(lines[origin.TopRow-1+3]))
			if got := line3Runes[origin.LeftCol-1+8]; got != 'b' {
				t.Fatalf("size %v expected 'b' of blade at stage (3,8), got %q", size, got)
			}
			line4Runes := []rune(stripANSI(lines[origin.TopRow-1+4]))
			if got := line4Runes[origin.LeftCol-1+15]; got != 'B' {
				t.Fatalf("size %v expected 'B' of Blend at stage (4,15), got %q", size, got)
			}

			if confirm == confirmSaveExit {
				joined := stripANSI(strings.Join(lines, "\n"))
				if !strings.Contains(joined, "Override app/tool cursor colors? [y/N]") || !strings.Contains(joined, "Override app/tool colors (opencode, nvim)? [y/N]") {
					t.Fatalf("size %v missing color [y/N] prompt in rendered frame:\n%s", size, joined)
				}
			}
			if confirm == confirmSaveExitShapes {
				joined := stripANSI(strings.Join(lines, "\n"))
				if !strings.Contains(joined, "Lock cursor shape in apps/TUIs? [y/N]") || !strings.Contains(joined, "Lock cursor shape in apps (opencode, nvim)? [y/N]") {
					t.Fatalf("size %v missing shape [y/N] prompt in rendered frame:\n%s", size, joined)
				}
			}
		}
	}
}

func TestSelectionRowKeepsUnbrokenBackground(t *testing.T) {
	row := renderListItem(20, "liquid", true, true, true, ansiBlue)
	if VisibleLen(row) != 20 {
		t.Fatalf("expected width 20, got %d", VisibleLen(row))
	}
	firstReset := strings.Index(row, ansiReset)
	if firstReset != len(row)-len(ansiReset) {
		t.Fatalf("ansiReset appeared mid-row at byte %d of %d: %q", firstReset, len(row), row)
	}
}

func TestStageSurgicalWordDeleteAndTyping(t *testing.T) {
	stage := NewLiveStage()
	origin := StageOrigin{TopRow: 4, LeftCol: 45, Cols: 46, Rows: 7}

	stage.StageForDemo(DemoWordDel, AllShapes[0])
	if stage.CurRow != 3 || stage.CurCol != stageEndCol || !stage.WordVisible {
		t.Fatalf("unexpected pre-stage state: %+v", stage)
	}

	seq, _ := stage.StepNext(origin)
	if !strings.Contains(seq, "\x1b[7;69H\x1b[14X") {
		t.Fatalf("expected surgical ECH sequence, got %q", seq)
	}
	if stage.WordVisible || stage.CurCol != stagePrefixCol {
		t.Fatalf("expected word hidden and cursor at %d, got %+v", stagePrefixCol, stage)
	}
}

func TestConfigApplyHollowAndOverrideColorsMacrosAndRestore(t *testing.T) {
	tmp := t.TempDir()
	shadersDir := filepath.Join(tmp, "shaders")
	if err := os.MkdirAll(shadersDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range AllEffects {
		if err := os.WriteFile(filepath.Join(shadersDir, e.ShaderFile), []byte("// shader\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Test hex cursor-color parser (#39ff14 neon green)
	r, g, b, ok := ParseHexCursorColor("# comment\nfont-size = 14\ncursor-color = #39ff14\n")
	if !ok || math.Abs(r-57.0/255.0) > 1e-4 || math.Abs(g-1.0) > 1e-4 || math.Abs(b-20.0/255.0) > 1e-4 {
		t.Fatalf("unexpected parsed RGB: (%v, %v, %v, %v)", r, g, b, ok)
	}

	// Start with neon + bar + overrideColors=true + overrideShapes=true (explicit #39ff14)
	rgb := [3]float64{r, g, b}
	if err := writeSnippetWithColor(tmp, AllEffects[FindEffectIndex("neon")], AllShapes[FindShapeIndex("bar")], false, true, true, &rgb); err != nil {
		t.Fatal(err)
	}
	saved := ReadActiveState(tmp)
	if saved.Effect != EffectNeon || saved.Shape != ShapeBar || !saved.OverrideColors || !saved.OverrideShapes {
		t.Fatalf("unexpected saved state: %+v", saved)
	}
	genBar, err := os.ReadFile(filepath.Join(tmp, "configs", ".gen-neon-bar.glsl"))
	if err != nil {
		t.Fatalf("expected .gen-neon-bar.glsl when overrides=true: %v", err)
	}
	if !strings.Contains(string(genBar), "#define iCurrentCursorColor vec4(0.223529, 1.000000, 0.078431, 1.0)") {
		t.Fatalf("missing forced RGB macro in generated shader: %s", string(genBar))
	}
	if !strings.Contains(string(genBar), "#define _GCC_TARGET_STYLE CURSORSTYLE_BAR") || !strings.Contains(string(genBar), "_gccLockCursorBox") {
		t.Fatalf("missing shape lock macros in generated shader: %s", string(genBar))
	}

	// Fallback when no hex cursor-color is set (theme color via iCursorColor), overrideShapes=false
	if err := writeSnippetWithColor(tmp, AllEffects[FindEffectIndex("katana")], AllShapes[FindShapeIndex("bar")], false, true, false, nil); err != nil {
		t.Fatal(err)
	}
	genTheme, err := os.ReadFile(filepath.Join(tmp, "configs", ".gen-katana-bar.glsl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(genTheme), "#define iCurrentCursorColor vec4(iCursorColor, 1.0)") {
		t.Fatalf("missing fallback iCursorColor macro: %s", string(genTheme))
	}
	if strings.Contains(string(genTheme), "_gccLockCursorBox") {
		t.Fatalf("unexpected shape lock macro when overrideShapes=false: %s", string(genTheme))
	}

	// Switch to liquid + hollow (overrideColors=false, overrideShapes=false) and verify hollow macro without overrides
	if err := writeSnippetWithColor(tmp, AllEffects[FindEffectIndex("liquid")], AllShapes[FindShapeIndex("hollow")], false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	genHollow, err := os.ReadFile(filepath.Join(tmp, "configs", ".gen-liquid-hollow.glsl"))
	if err != nil {
		t.Fatalf("expected .gen-liquid-hollow.glsl: %v", err)
	}
	if !strings.Contains(string(genHollow), "#define CURSORSTYLE_BLOCK_HOLLOW (iCurrentCursorStyle)") {
		t.Fatalf("missing hollow runtime macro in generated shader: %s", string(genHollow))
	}
	if strings.Contains(string(genHollow), "#define iCurrentCursorColor") {
		t.Fatalf("unexpected color override macro when overrideColors=false: %s", string(genHollow))
	}

	// Verify ShapeDECSCUSR mappings
	if got := ShapeDECSCUSR(AllShapes[FindShapeIndex("underscore")], false); got != "\x1b[4 q" {
		t.Fatalf("expected \\x1b[4 q for underscore, got %q", got)
	}
	if got := ShapeDECSCUSR(AllShapes[FindShapeIndex("bar")], false); got != "\x1b[6 q" {
		t.Fatalf("expected \\x1b[6 q for bar, got %q", got)
	}
	if got := ShapeDECSCUSR(AllShapes[FindShapeIndex("hollow")], false); got != "\x1b[2 q" {
		t.Fatalf("expected \\x1b[2 q for hollow, got %q", got)
	}

	// Revert to saved (neon + bar + OverrideColors=true + OverrideShapes=true)
	if err := RestoreState(tmp, saved); err != nil {
		t.Fatal(err)
	}
	reverted := ReadActiveState(tmp)
	if reverted.Effect != EffectNeon || reverted.Shape != ShapeBar || !reverted.OverrideColors || !reverted.OverrideShapes || reverted.Off {
		t.Fatalf("unexpected reverted state: %+v", reverted)
	}
}

func TestParseGhosttyPIDsFromPS(t *testing.T) {
	psSample := `
    1     0 /sbin/launchd
 4120     1 /Applications/Ghostty.app/Contents/MacOS/ghostty
 4125  4120 /bin/zsh
 4900  4125 /Users/dev/projects/ghosttylib-cursor-control/cursor-picker
 8811     1 /Users/dev/My Apps/Ghostty.app/Contents/MacOS/ghostty
`
	pids := parseGhosttyPIDsFromPS(psSample, 4900)
	if len(pids) != 2 || pids[0] != 4120 || pids[1] != 8811 {
		t.Fatalf("unexpected Ghostty PIDs: %v (want [4120 8811])", pids)
	}
}

func TestParseKeysFastKeyRepeat(t *testing.T) {
	got := parseKeys([]byte("\x1b[B\x1b[B\x1b[Zjw\r\x1b"))
	want := []string{"\x1b[B", "\x1b[B", "\x1b[Z", "j", "w", "\r", "\x1b"}
	if len(got) != len(want) {
		t.Fatalf("parseKeys len=%d (%q), want %d (%q)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
