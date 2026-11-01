package picker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// ActiveState represents what is currently configured in configs/ghostty-cursor.conf.
type ActiveState struct {
	Off      bool
	Effect   EffectID
	Shape    ShapeID
	Linked   bool
	RepoRoot string
}

// ResolveRepoRoot locates the ghosttylib-cursor-control repository root.
func ResolveRepoRoot() (string, error) {
	if env := os.Getenv("GHOSTTYLIB_CURSOR_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		if hasShadersDir(exeDir) {
			return exeDir, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for i := 0; i < 4; i++ {
		if hasShadersDir(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, nil
}

func hasShadersDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "shaders"))
	return err == nil && st.IsDir()
}

func ghosttyConfigPath() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	home, _ := os.UserHomeDir()
	xdgCfg := filepath.Join(home, ".config", "ghostty", "config")
	if xdg != "" {
		xdgCfg = filepath.Join(xdg, "ghostty", "config")
	}
	macCfg := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config")
	if xdg == "" {
		if _, err := os.Stat(xdgCfg); os.IsNotExist(err) {
			if _, err2 := os.Stat(macCfg); err2 == nil {
				return macCfg
			}
		}
	}
	return xdgCfg
}

// ReadActiveState parses configs/ghostty-cursor.conf and checks Ghostty's config link.
func ReadActiveState(repoRoot string) ActiveState {
	st := ActiveState{
		Effect:   EffectKatana,
		Shape:    ShapeSlash,
		RepoRoot: repoRoot,
	}
	snippet := filepath.Join(repoRoot, "configs", "ghostty-cursor.conf")
	data, err := os.ReadFile(snippet)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# Preset:") {
				rest := strings.TrimSpace(strings.TrimPrefix(line, "# Preset:"))
				parts := strings.Split(rest, "|")
				if len(parts) >= 1 {
					p := strings.TrimSpace(parts[0])
					if p == "off" {
						st.Off = true
					} else {
						st.Effect = AllEffects[FindEffectIndex(p)].ID
					}
				}
				if len(parts) >= 2 {
					s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "Shape:"))
					if s != "" && s != "native" {
						st.Shape = AllShapes[FindShapeIndex(s)].ID
					}
				}
				break
			}
		}
	}

	gcfg := ghosttyConfigPath()
	includeLine := "config-file = ?" + snippet
	if gdata, err := os.ReadFile(gcfg); err == nil {
		for _, line := range strings.Split(string(gdata), "\n") {
			if strings.TrimSpace(line) == includeLine {
				st.Linked = true
				break
			}
		}
	}
	return st
}

// EnsureGhosttyLinked ensures ~/.config/ghostty/config includes configs/ghostty-cursor.conf.
func EnsureGhosttyLinked(repoRoot string) error {
	st := ReadActiveState(repoRoot)
	if st.Linked {
		return nil
	}
	gcfg := ghosttyConfigPath()
	if err := os.MkdirAll(filepath.Dir(gcfg), 0o755); err != nil {
		return err
	}
	snippet := filepath.Join(repoRoot, "configs", "ghostty-cursor.conf")
	includeLine := "config-file = ?" + snippet

	existing, _ := os.ReadFile(gcfg)
	var kept []string
	for _, line := range strings.Split(string(existing), "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "# ghosttylib-cursor-control" || trimmed == includeLine {
			continue
		}
		if strings.HasPrefix(trimmed, "config-file = ?") && strings.HasSuffix(trimmed, "/configs/ghostty-cursor.conf") {
			continue
		}
		kept = append(kept, trimmed)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	var out strings.Builder
	if len(kept) > 0 {
		out.WriteString(strings.Join(kept, "\n"))
		out.WriteString("\n\n")
	}
	out.WriteString("# ghosttylib-cursor-control\n")
	out.WriteString(includeLine + "\n")
	return os.WriteFile(gcfg, []byte(out.String()), 0o644)
}

// ShapeDECSCUSR returns the runtime DECSCUSR escape sequence so an already-open Ghostty
// terminal surface immediately updates its cursor geometry (bar / underline / full-cell)
// without waiting for a new tab.
func ShapeDECSCUSR(shape ShapeInfo, off bool) string {
	if off {
		return "\x1b[0 q"
	}
	switch shape.NativeStyle {
	case StyleBar:
		return "\x1b[6 q" // Steady vertical bar (2px x cellH)
	case StyleUnderline:
		return "\x1b[4 q" // Steady horizontal underline (cellW x 2px)
	default:
		// StyleBlock and StyleHollow (including brackets) use full-cell geometry (cellW x cellH);
		// hollow & brackets shaders distinguish themselves via generated #define macros.
		return "\x1b[2 q"
	}
}

// ApplySelection writes configs/ghostty-cursor.conf directly in <0.2ms and ensures Ghostty is linked.
func ApplySelection(repoRoot string, effect EffectInfo, shape ShapeInfo) error {
	if err := writeSnippetDirect(repoRoot, effect, shape, false); err != nil {
		return err
	}
	_ = EnsureGhosttyLinked(repoRoot)
	return nil
}

// ApplyOff disables custom cursor shaders (stock native Ghostty cursor).
func ApplyOff(repoRoot string) error {
	return writeSnippetDirect(repoRoot, AllEffects[0], AllShapes[0], true)
}

// RestoreState restores a previously saved ActiveState back to configs/ghostty-cursor.conf.
func RestoreState(repoRoot string, st ActiveState) error {
	if st.Off {
		return ApplyOff(repoRoot)
	}
	eff := AllEffects[FindEffectIndex(string(st.Effect))]
	shp := AllShapes[FindShapeIndex(string(st.Shape))]
	return writeSnippetDirect(repoRoot, eff, shp, false)
}

var (
	ghosttyPIDMu   sync.Mutex
	cachedPIDs     []int
	pidsDiscovered bool
)

// parseGhosttyPIDsFromPS parses `ps -axo pid=,ppid=,comm=` output and returns all Ghostty PIDs,
// prioritizing the ancestor Ghostty instance hosting our current process.
func parseGhosttyPIDsFromPS(psOut string, selfPID int) []int {
	type procRow struct {
		ppid int
		comm string
	}
	procs := make(map[int]procRow)
	var allMatches []int

	for _, rawLine := range strings.Split(psOut, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		// Extract field 1 (pid), field 2 (ppid), and remainder (comm path, which may contain spaces)
		sp1 := strings.IndexAny(line, " \t")
		if sp1 <= 0 {
			continue
		}
		pid, err1 := strconv.Atoi(line[:sp1])
		rest := strings.TrimLeft(line[sp1:], " \t")
		sp2 := strings.IndexAny(rest, " \t")
		if err1 != nil || sp2 <= 0 {
			continue
		}
		ppid, err2 := strconv.Atoi(rest[:sp2])
		if err2 != nil {
			continue
		}
		comm := strings.TrimSpace(rest[sp2:])
		procs[pid] = procRow{ppid: ppid, comm: comm}

		if pid != selfPID {
			base := filepath.Base(comm)
			if strings.EqualFold(base, "ghostty") || strings.Contains(comm, "Ghostty.app/Contents/MacOS/") {
				allMatches = append(allMatches, pid)
			}
		}
	}

	seen := make(map[int]bool)
	var ordered []int

	// 1. Walk ancestor chain from selfPID -> 1 first
	cur := selfPID
	for hops := 0; hops < 16 && cur > 1; hops++ {
		pr, ok := procs[cur]
		if !ok {
			break
		}
		if cur != selfPID {
			base := filepath.Base(pr.comm)
			if strings.EqualFold(base, "ghostty") || strings.Contains(pr.comm, "Ghostty.app/Contents/MacOS/") {
				seen[cur] = true
				ordered = append(ordered, cur)
			}
		}
		if pr.ppid == cur {
			break
		}
		cur = pr.ppid
	}

	// 2. Include any other running Ghostty app PIDs
	for _, pid := range allMatches {
		if !seen[pid] {
			seen[pid] = true
			ordered = append(ordered, pid)
		}
	}
	return ordered
}

// FindGhosttyPIDs locates running Ghostty processes (including macOS Ghostty.app bundles
// where full path argv[0] causes `pkill -x ghostty` to miss).
func FindGhosttyPIDs() []int {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,comm=").Output()
	if err != nil {
		return nil
	}
	return parseGhosttyPIDsFromPS(string(out), os.Getpid())
}

// ReloadGhostty sends SIGUSR2 directly via syscall.Kill to running Ghostty PIDs so
// Ghostty hot-reloads configs/ghostty-cursor.conf and recompiles the GPU shader in milliseconds.
func ReloadGhostty() int {
	ghosttyPIDMu.Lock()
	defer ghosttyPIDMu.Unlock()

	if !pidsDiscovered || len(cachedPIDs) == 0 {
		cachedPIDs = FindGhosttyPIDs()
		pidsDiscovered = true
	}

	signaled := 0
	needRescan := false
	for _, pid := range cachedPIDs {
		if err := syscall.Kill(pid, syscall.SIGUSR2); err == nil {
			signaled++
		} else {
			needRescan = true
		}
	}

	if needRescan {
		cachedPIDs = FindGhosttyPIDs()
		signaled = 0
		for _, pid := range cachedPIDs {
			if err := syscall.Kill(pid, syscall.SIGUSR2); err == nil {
				signaled++
			}
		}
	}
	return signaled
}

func writeSnippetDirect(repoRoot string, effect EffectInfo, shape ShapeInfo, off bool) error {
	cfgDir := filepath.Join(repoRoot, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	if matches, _ := filepath.Glob(filepath.Join(cfgDir, ".gen-*.glsl")); matches != nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	snippet := filepath.Join(cfgDir, "ghostty-cursor.conf")
	if off {
		content := "# Managed by ghosttylib-cursor-control (./cursorctl)\n# Preset: off | Shape: native\n"
		return os.WriteFile(snippet, []byte(content), 0o644)
	}

	baseShader := filepath.Join(repoRoot, "shaders", effect.ShaderFile)
	activeShader := baseShader
	isPureHollow := shape.NativeStyle == StyleBlockHollow && !shape.BracketsSDF
	if shape.SlashSweep || shape.BracketsSDF || isPureHollow {
		slashVal := 0
		if shape.SlashSweep {
			slashVal = 1
		}
		bracketsVal := 0
		if shape.BracketsSDF {
			bracketsVal = 1
		}
		rawShader, err := os.ReadFile(baseShader)
		if err != nil {
			return err
		}
		activeShader = filepath.Join(cfgDir, fmt.Sprintf(".gen-%s-%s.glsl", effect.ID, shape.ID))
		var hdr strings.Builder
		fmt.Fprintf(&hdr, "#define SLASH_SWEEP_MODE %d\n#define BRACKETS_SHAPE_MODE %d\n", slashVal, bracketsVal)
		if isPureHollow {
			// Ensure hollow wireframe renders identically both on new tabs (CURSORSTYLE_BLOCK_HOLLOW)
			// and on already-open tabs switched live via DECSCUSR \x1b[2 q (full-cell geometry).
			hdr.WriteString("#undef CURSORSTYLE_BLOCK_HOLLOW\n#define CURSORSTYLE_BLOCK_HOLLOW (iCurrentCursorStyle)\n")
			hdr.WriteString("#undef CURSORSTYLE_BLOCK\n#define CURSORSTYLE_BLOCK (-99)\n")
		}
		hdr.Write(rawShader)
		if err := os.WriteFile(activeShader, []byte(hdr.String()), 0o644); err != nil {
			return err
		}
	}

	content := fmt.Sprintf(`# Managed by ghosttylib-cursor-control (./cursorctl)
# Preset: %s | Shape: %s

# Native Ghostty geometry: bar | block | underline | block_hollow
cursor-style = %s

# Prevent shell integration from emitting blinking DECSCUSR (\e[5 q / \e[1 q) that fights the shader
shell-integration-features = no-cursor

# Pass Cmd+Z / Cmd+Shift+Z through to terminal TUI apps instead of
# Ghostty's default macOS Edit -> Undo menu shortcut (Cmd+Shift+T still reopens closed tabs)
keybind = super+z=unbind
keybind = super+shift+z=unbind

# Disable Ghostty's hard binary blink so the GLSL shader controls the smooth animation cycle
cursor-style-blink = false

# Hide Ghostty's static native cursor so only the animated shader cursor is visible,
# while keeping the character under block cursors rendered in its normal foreground color
cursor-opacity = 0
cursor-text = cell-foreground

# Animate smoothly while Ghostty is focused (0%% idle CPU when unfocused)
custom-shader-animation = true

# Active cursor shader
custom-shader = %s
`, effect.ID, shape.ID, shape.GhosttyStyle, activeShader)

	return os.WriteFile(snippet, []byte(content), 0o644)
}
