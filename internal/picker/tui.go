package picker

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiNoBold    = "\x1b[22m"
	ansiDim       = "\x1b[38;2;115;126;152m"
	ansiMuted     = "\x1b[38;2;148;160;188m"
	ansiText      = "\x1b[38;2;222;230;245m"
	ansiBorder    = "\x1b[38;2;58;68;94m"
	ansiWinBorder = "\x1b[38;2;46;54;78m"
	ansiBorderAct = "\x1b[38;2;110;168;255m"
	ansiCyan      = "\x1b[38;2;86;208;248m"
	ansiBlue      = "\x1b[38;2;122;162;247m"
	ansiGreen     = "\x1b[38;2;74;222;128m"
	ansiAmber     = "\x1b[38;2;251;191;36m"
	ansiRedDot    = "\x1b[38;2;247;118;142m"
	ansiYelDot    = "\x1b[38;2;224;175;104m"
	ansiGrnDot    = "\x1b[38;2;158;206;106m"
	ansiSelectBg  = "\x1b[48;2;36;48;78m"
	ansiSelectDim = "\x1b[48;2;24;30;46m"
	ansiPromptBg  = "\x1b[48;2;30;41;68m"
)

type FocusCol int

const (
	FocusShapes FocusCol = iota
	FocusEffects
)

type confirmPromptMode int

const (
	confirmNone confirmPromptMode = iota
	confirmSaveExit
	confirmSaveStay
)

// Result describes the outcome when exiting cursor-picker.
type Result struct {
	Saved          bool
	TurnedOff      bool
	Reverted       bool
	OverrideColors bool
	Effect         EffectInfo
	Shape          ShapeInfo
}

type App struct {
	repoRoot         string
	savedState       ActiveState
	shapeIdx         int
	effectIdx        int
	focus            FocusCol
	previewOff       bool
	ghosttyConnected bool
	stage            *LiveStage
	statusNotice     string
	confirmMode      confirmPromptMode
}

func NewApp(repoRoot string) *App {
	saved := ReadActiveState(repoRoot)
	sIdx := FindShapeIndex(string(saved.Shape))
	eIdx := FindEffectIndex(string(saved.Effect))
	st := NewLiveStage()
	st.StageForDemo(AllEffects[eIdx].InitialDemo, AllShapes[sIdx])
	return &App{
		repoRoot:         repoRoot,
		savedState:       saved,
		shapeIdx:         sIdx,
		effectIdx:        eIdx,
		focus:            FocusShapes,
		previewOff:       saved.Off,
		ghosttyConnected: true,
		stage:            st,
	}
}

// NewSnapshotApp constructs an App configured for a specific shape, effect, focus pane, and stage word state.
func NewSnapshotApp(repoRoot string, shape ShapeID, effect EffectID, focus FocusCol, wordVisible bool) *App {
	app := NewApp(repoRoot)
	app.savedState = ActiveState{Effect: EffectKatana, Shape: ShapeSlash, Linked: true, RepoRoot: repoRoot}
	app.shapeIdx = FindShapeIndex(string(shape))
	app.effectIdx = FindEffectIndex(string(effect))
	app.focus = focus
	app.previewOff = false
	app.ghosttyConnected = true
	app.stage.WordVisible = wordVisible
	return app
}

// VisibleLen returns the printable rune count of s, ignoring ANSI CSI escape sequences.
func VisibleLen(s string) int {
	n := 0
	inEsc := false
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			inEsc = true
			i++
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			i++
			continue
		}
		_, sz := utf8.DecodeRuneInString(s[i:])
		n++
		i += sz
	}
	return n
}

// FitCell pads with spaces or truncates (preserving ANSI resets) to exact width visible runes.
func FitCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	v := VisibleLen(s)
	if v == width {
		return s
	}
	if v < width {
		return s + strings.Repeat(" ", width-v)
	}
	var sb strings.Builder
	count := 0
	inEsc := false
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			inEsc = true
			sb.WriteByte(s[i])
			i++
			continue
		}
		if inEsc {
			sb.WriteByte(s[i])
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			i++
			continue
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if count >= width {
			break
		}
		sb.WriteRune(r)
		count++
		i += sz
	}
	sb.WriteString(ansiReset)
	if count < width {
		sb.WriteString(strings.Repeat(" ", width-count))
	}
	return sb.String()
}

// renderListItem renders a clean raw-name row with an unbroken full-width highlight bar when selected.
func renderListItem(width int, label string, isSelected, isFocused, isSaved bool, accentFG string) string {
	ptr := " "
	ptrFG := ansiDim
	rowBG := ""
	labelStyle := ansiText

	if isSelected {
		if isFocused {
			ptr = "▸"
			ptrFG = accentFG
			rowBG = ansiSelectBg
			labelStyle = ansiBold + ansiText
		} else {
			ptr = "›"
			ptrFG = ansiMuted
			rowBG = ansiSelectDim
			labelStyle = ansiBold + ansiText
		}
	}

	act := " "
	actFG := ansiDim
	if isSaved {
		act = "•"
		actFG = ansiGreen
	}

	visLen := 4 + utf8.RuneCountInString(label)
	padRight := maxInt(0, width-visLen)

	var sb strings.Builder
	if rowBG != "" {
		sb.WriteString(rowBG)
	}
	sb.WriteByte(' ')
	sb.WriteString(ptrFG)
	sb.WriteString(ptr)
	sb.WriteString(actFG)
	sb.WriteString(act)
	sb.WriteByte(' ')
	sb.WriteString(labelStyle)
	sb.WriteString(label)
	sb.WriteString(ansiNoBold)
	if padRight > 0 {
		sb.WriteString(strings.Repeat(" ", padRight))
	}
	sb.WriteString(ansiReset)
	return FitCell(sb.String(), width)
}

func wrapPlain(text string, width int) (string, string) {
	if width <= 4 {
		return FitCell(text, width), ""
	}
	words := strings.Fields(text)
	var l1, l2 strings.Builder
	onSecond := false
	for _, w := range words {
		if !onSecond {
			need := len(w)
			if l1.Len() > 0 {
				need++
			}
			if l1.Len()+need <= width {
				if l1.Len() > 0 {
					l1.WriteByte(' ')
				}
				l1.WriteString(w)
				continue
			}
			onSecond = true
		}
		need := len(w)
		if l2.Len() > 0 {
			need++
		}
		if l2.Len()+need <= width {
			if l2.Len() > 0 {
				l2.WriteByte(' ')
			}
			l2.WriteString(w)
		} else {
			break
		}
	}
	return l1.String(), l2.String()
}

func renderPromptBannerRow(content string, width int) string {
	vis := VisibleLen(content)
	pad := maxInt(0, width-vis)
	return FitCell(ansiPromptBg+content+strings.Repeat(" ", pad)+ansiReset, width)
}

// RenderFrame builds the 3-column TUI and returns (lines, StageOrigin).
func (a *App) RenderFrame(termCols, termRows int) ([]string, StageOrigin) {
	totalW := termCols
	if totalW < 86 {
		totalW = 86
	}
	if totalW > 126 {
		totalW = 126
	}

	wShapes := 18
	wEffects := 18
	if totalW >= 100 {
		wShapes = 20
		wEffects = 20
	}
	wPreview := totalW - 4 - wShapes - wEffects
	if wPreview < 44 {
		wPreview = 44
		totalW = 4 + wShapes + wEffects + wPreview
	}

	edRows := 7
	if termRows >= 26 {
		edRows = 8
	}
	edCols := wPreview - 4
	if edCols > 62 {
		edCols = 62
	}
	if edCols < 40 {
		edCols = 40
	}

	shape := AllShapes[a.shapeIdx]
	effect := AllEffects[a.effectIdx]

	var prevLines []string
	liveBadge := ansiGreen + "● LIVE GPU SHADER " + ansiReset
	if !a.ghosttyConnected {
		liveBadge = ansiAmber + "⚠ RUN INSIDE GHOSTTY " + ansiReset
	} else if a.previewOff {
		liveBadge = ansiAmber + "○ STOCK CURSOR " + ansiReset
	}
	hdrLeft := ansiBold + ansiCyan + " LIVE GHOSTTY STAGE" + ansiReset
	hdrGap := maxInt(1, wPreview-VisibleLen(hdrLeft)-VisibleLen(liveBadge))
	prevLines = append(prevLines, FitCell(hdrLeft+strings.Repeat(" ", hdrGap)+liveBadge, wPreview))

	padTotal := wPreview - (edCols + 2)
	padL := maxInt(0, padTotal/2)
	padR := maxInt(0, padTotal-padL)
	lMargin := strings.Repeat(" ", padL)
	rMargin := strings.Repeat(" ", padR)

	winTitle := " preview.go "
	winDashes := maxInt(1, edCols-8-len(winTitle))
	winTop := fmt.Sprintf("%s%s╭─ %s• %s• %s•%s %s%s%s%s╮%s%s",
		lMargin,
		ansiWinBorder,
		ansiRedDot, ansiYelDot, ansiGrnDot, ansiWinBorder,
		ansiMuted, winTitle,
		ansiWinBorder, strings.Repeat("─", winDashes),
		ansiReset,
		rMargin,
	)
	prevLines = append(prevLines, FitCell(winTop, wPreview))

	edLines := a.stage.RenderEditorLines(edCols, edRows)
	for r := 0; r < edRows; r++ {
		prevLines = append(prevLines, lMargin+ansiWinBorder+"│"+ansiReset+edLines[r]+ansiWinBorder+"│"+ansiReset+rMargin)
	}

	winBot := fmt.Sprintf("%s%s╰%s╯%s%s", lMargin, ansiWinBorder, strings.Repeat("─", edCols), ansiReset, rMargin)
	prevLines = append(prevLines, winBot)

	// Shape & Effect descriptions beneath the Live Stage
	prevLines = append(prevLines, FitCell(
		fmt.Sprintf(" %s%sShape:%s  %s%s%s %s— %s%s",
			ansiBold, ansiCyan, ansiReset,
			ansiBold+ansiText, shape.ID, ansiReset,
			ansiMuted, shape.Summary, ansiReset),
		wPreview,
	))

	eLine1, eLine2 := wrapPlain(string(effect.ID)+" — "+effect.Summary, wPreview-9)
	idPrefix := string(effect.ID) + " — "
	trimmedE1 := strings.TrimPrefix(eLine1, idPrefix)
	prevLines = append(prevLines, FitCell(
		fmt.Sprintf(" %s%sEffect:%s %s%s%s %s— %s%s",
			ansiBold, ansiBlue, ansiReset,
			ansiBold+ansiText, effect.ID, ansiReset,
			ansiMuted, trimmedE1, ansiReset),
		wPreview,
	))
	if eLine2 != "" && a.confirmMode == confirmNone {
		prevLines = append(prevLines, FitCell("         "+ansiMuted+eLine2+ansiReset, wPreview))
	}

	if a.confirmMode != confirmNone {
		p1 := fmt.Sprintf(" %s%s?%s %s%sOverride app/tool cursor colors? [y/N]%s",
			ansiBold, ansiAmber, ansiNoBold, ansiBold, ansiText, ansiNoBold)
		p2 := fmt.Sprintf("   %s%sy%s %sForce Ghostty%s  %s%sN/Enter%s %sKeep%s  %sEsc%s %scancel%s",
			ansiBold, ansiGreen, ansiNoBold, ansiText, ansiMuted,
			ansiBold, ansiCyan, ansiNoBold, ansiText, ansiMuted,
			ansiMuted, ansiNoBold, ansiDim, ansiNoBold)
		prevLines = append(prevLines, renderPromptBannerRow(p1, wPreview))
		prevLines = append(prevLines, renderPromptBannerRow(p2, wPreview))
	} else if a.stage.Paused {
		prevLines = append(prevLines, FitCell(" "+ansiAmber+"Motion paused (idle inspection — press p to resume)"+ansiReset, wPreview))
	} else if a.statusNotice != "" {
		prevLines = append(prevLines, FitCell(" "+ansiGreen+a.statusNotice+ansiReset, wPreview))
	}

	// SHAPES column (clean raw names ONLY)
	var shapeLines []string
	shHdrStyle := ansiMuted
	shPtr := " "
	if a.focus == FocusShapes {
		shHdrStyle = ansiBold + ansiCyan
		shPtr = "▸"
	}
	shapeLines = append(shapeLines, FitCell(fmt.Sprintf(" %s%s SHAPES (%d)%s", shHdrStyle, shPtr, len(AllShapes), ansiReset), wShapes))
	shapeLines = append(shapeLines, ansiBorder+" "+strings.Repeat("─", wShapes-2)+" "+ansiReset)
	for i, s := range AllShapes {
		isSaved := !a.savedState.Off && a.savedState.Shape == s.ID
		shapeLines = append(shapeLines, renderListItem(wShapes, string(s.ID), i == a.shapeIdx, a.focus == FocusShapes, isSaved, ansiCyan))
	}
	shapeLines = append(shapeLines, strings.Repeat(" ", wShapes))
	shapeLines = append(shapeLines, FitCell(" "+ansiGreen+"•"+ansiDim+" initial saved"+ansiReset, wShapes))

	// EFFECTS column (clean raw names ONLY)
	var effectLines []string
	efHdrStyle := ansiMuted
	efPtr := " "
	if a.focus == FocusEffects {
		efHdrStyle = ansiBold + ansiBlue
		efPtr = "▸"
	}
	effectLines = append(effectLines, FitCell(fmt.Sprintf(" %s%s EFFECTS (%d)%s", efHdrStyle, efPtr, len(AllEffects), ansiReset), wEffects))
	effectLines = append(effectLines, ansiBorder+" "+strings.Repeat("─", wEffects-2)+" "+ansiReset)
	for i, e := range AllEffects {
		isSaved := !a.savedState.Off && a.savedState.Effect == e.ID
		effectLines = append(effectLines, renderListItem(wEffects, string(e.ID), i == a.effectIdx, a.focus == FocusEffects, isSaved, ansiBlue))
	}

	bodyRows := len(effectLines)
	if len(shapeLines) > bodyRows {
		bodyRows = len(shapeLines)
	}
	if len(prevLines) > bodyRows {
		bodyRows = len(prevLines)
	}

	var out []string

	savedLabel := fmt.Sprintf("%s + %s", a.savedState.Effect, a.savedState.Shape)
	if a.savedState.Off {
		savedLabel = "off (stock)"
	} else if a.savedState.OverrideColors {
		savedLabel += " [override]"
	}
	titleLeft := " ghosttylib-cursor-control "
	titleRight := fmt.Sprintf(" Saved: %s ", savedLabel)
	dashCount := maxInt(1, totalW-2-len(titleLeft)-len(titleRight)-2)
	topBar := fmt.Sprintf("%s╭─%s%s%s%s%s%s%s─╮%s",
		ansiBorderAct,
		ansiBold+ansiCyan, titleLeft, ansiReset,
		ansiBorderAct, strings.Repeat("─", dashCount),
		ansiMuted, titleRight,
		ansiBorderAct,
	) + ansiReset
	out = append(out, topBar)

	vSep := ansiBorder + "│" + ansiReset
	outerV := ansiBorderAct + "│" + ansiReset
	for r := 0; r < bodyRows; r++ {
		c1 := strings.Repeat(" ", wShapes)
		if r < len(shapeLines) {
			c1 = shapeLines[r]
		}
		c2 := strings.Repeat(" ", wEffects)
		if r < len(effectLines) {
			c2 = effectLines[r]
		}
		c3 := strings.Repeat(" ", wPreview)
		if r < len(prevLines) {
			c3 = prevLines[r]
		}
		out = append(out, outerV+c3OrCols(c1, vSep, c2, c3)+outerV)
	}

	divBar := fmt.Sprintf("%s├%s┴%s┴%s┤%s",
		ansiBorderAct,
		strings.Repeat("─", wShapes),
		strings.Repeat("─", wEffects),
		strings.Repeat("─", wPreview),
		ansiReset,
	)
	out = append(out, divBar)

	var helpLine string
	if a.confirmMode != confirmNone {
		helpLine = fmt.Sprintf(
			" %s%sOverride app/tool colors (opencode, nvim)? [y/N]%s  %s%sy%s %sYes%s  %s%sN/Enter%s %sNo%s  %sEsc%s %scancel%s",
			ansiAmber, ansiBold, ansiReset,
			ansiGreen, ansiBold, ansiReset, ansiText, ansiReset,
			ansiCyan, ansiBold, ansiReset, ansiText, ansiReset,
			ansiMuted, ansiReset, ansiDim, ansiReset,
		)
	} else {
		helpLine = fmt.Sprintf(
			" %s←/→ Tab%s %spane%s  %s↑/↓ j/k%s %slive preview%s  %sSpace%s %sjump%s  %sw%s %sword-del%s  %sv%s %sline%s  %sp%s %spause%s  %sEnter%s %ssave & exit%s  %so%s %soff%s  %sEsc/q%s %srevert%s",
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiCyan, ansiReset, ansiDim, ansiReset,
			ansiGreen+ansiBold, ansiReset, ansiDim, ansiReset,
			ansiAmber, ansiReset, ansiDim, ansiReset,
			ansiMuted, ansiReset, ansiDim, ansiReset,
		)
	}
	out = append(out, outerV+FitCell(helpLine, totalW-2)+outerV)

	botBar := fmt.Sprintf("%s╰%s╯%s", ansiBorderAct, strings.Repeat("─", totalW-2), ansiReset)
	out = append(out, botBar)

	origin := StageOrigin{
		TopRow:  4,
		LeftCol: 5 + wShapes + wEffects + padL,
		Cols:    edCols,
		Rows:    edRows,
	}
	return out, origin
}

func c3OrCols(c1, vSep, c2, c3 string) string {
	return c1 + vSep + c2 + vSep + c3
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// parseKeys splits a raw stdin buffer into individual key / CSI / SS3 sequences
// so rapid macOS key-repeat bursts (e.g. "\x1b[B\x1b[B") are never dropped.
func parseKeys(b []byte) []string {
	var out []string
	for i := 0; i < len(b); {
		if b[i] == 0x1b {
			if i+2 < len(b) && (b[i+1] == '[' || b[i+1] == 'O') {
				end := i + 2
				for end < len(b) && (b[end] < 0x40 || b[end] > 0x7e) {
					end++
				}
				if end < len(b) {
					out = append(out, string(b[i:end+1]))
					i = end + 1
					continue
				}
			}
			if i+1 == len(b) {
				out = append(out, "\x1b")
				i++
				continue
			}
			i += 2
			continue
		}
		_, sz := utf8.DecodeRune(b[i:])
		if sz <= 0 {
			sz = 1
		}
		out = append(out, string(b[i:i+sz]))
		i += sz
	}
	return out
}

// RunInteractive runs the live Ghostty GPU cursor picker.
// Every highlighted Shape x Effect updates cursor geometry via DECSCUSR immediately and
// hot-loads the real GLSL shader into Ghostty via direct SIGUSR2!
func RunInteractive(repoRoot string) (Result, error) {
	inFD := int(os.Stdin.Fd())
	outFD := int(os.Stdout.Fd())
	if !IsTerminal(inFD) || !IsTerminal(outFD) {
		return Result{}, fmt.Errorf("cursor-picker requires an interactive Ghostty terminal")
	}

	app := NewApp(repoRoot)
	_ = EnsureGhosttyLinked(repoRoot)
	app.ghosttyConnected = len(FindGhosttyPIDs()) > 0

	exitDECSCUSR := ShapeDECSCUSR(AllShapes[FindShapeIndex(string(app.savedState.Shape))], app.savedState.Off)

	oldState, err := MakeRaw(inFD)
	if err != nil {
		return Result{}, fmt.Errorf("failed to enable raw terminal mode: %w", err)
	}

	initSeq := ShapeDECSCUSR(AllShapes[app.shapeIdx], app.previewOff)
	_, _ = io.WriteString(os.Stdout, "\x1b[?1049h"+initSeq+"\x1b[?25h\x1b[2J\x1b[H")
	cleanup := func() {
		_, _ = io.WriteString(os.Stdout, "\x1b[?2026l\x1b[0m\x1b[?1049l"+exitDECSCUSR+"\x1b[?25h")
		RestoreTerm(inFD, oldState)
	}
	defer cleanup()

	keyCh := make(chan string, 64)
	go func() {
		buf := make([]byte, 64)
		for {
			n, rerr := os.Stdin.Read(buf)
			if rerr != nil || n == 0 {
				close(keyCh)
				return
			}
			for _, tok := range parseKeys(buf[:n]) {
				keyCh <- tok
			}
		}
	}()

	winchCh := make(chan os.Signal, 4)
	signal.Notify(winchCh, syscall.SIGWINCH)
	defer signal.Stop(winchCh)

	var currentOrigin StageOrigin
	needClear := true

	drawUI := func() {
		cols, rows := GetTermSize(outFD)
		lines, origin := app.RenderFrame(cols, rows)
		currentOrigin = origin

		var frame strings.Builder
		frame.Grow(12288)
		frame.WriteString("\x1b[?2026h")
		if needClear {
			frame.WriteString("\x1b[2J")
			needClear = false
		}
		maxLines := len(lines)
		if maxLines > rows {
			maxLines = rows
		}
		for i := 0; i < maxLines; i++ {
			fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K%s", i+1, lines[i])
		}
		frame.WriteString(ShapeDECSCUSR(AllShapes[app.shapeIdx], app.previewOff))
		sr, sc := origin.ScreenPos(app.stage.CurRow, app.stage.CurCol)
		fmt.Fprintf(&frame, "\x1b[%d;%dH\x1b[?2026l", sr, sc)
		_, _ = io.WriteString(os.Stdout, frame.String())
	}

	reloadTimer := time.NewTimer(time.Hour)
	reloadTimer.Stop()
	defer reloadTimer.Stop()

	stepTimer := time.NewTimer(500 * time.Millisecond)
	defer stepTimer.Stop()

	resetTimer := func(t *time.Timer, d time.Duration) {
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(d)
	}

	scheduleLiveReload := func() {
		if !stepTimer.Stop() {
			select {
			case <-stepTimer.C:
			default:
			}
		}
		resetTimer(reloadTimer, 35*time.Millisecond)
	}

	commitSave := func(overrideColors bool, exitAfter bool) (Result, bool, error) {
		eff := AllEffects[app.effectIdx]
		shp := AllShapes[app.shapeIdx]
		if err := ApplySelection(app.repoRoot, eff, shp, overrideColors); err != nil {
			return Result{}, false, err
		}
		ReloadGhostty()
		exitDECSCUSR = ShapeDECSCUSR(shp, false)
		app.savedState.Off = false
		app.savedState.Effect = eff.ID
		app.savedState.Shape = shp.ID
		app.savedState.OverrideColors = overrideColors
		app.confirmMode = confirmNone
		if exitAfter {
			return Result{
				Saved:          true,
				OverrideColors: overrideColors,
				Effect:         eff,
				Shape:          shp,
			}, true, nil
		}
		modeLabel := "keep tool colors"
		if overrideColors {
			modeLabel = "override tool colors"
		}
		app.statusNotice = fmt.Sprintf("Saved: %s + %s (%s)", eff.ID, shp.ID, modeLabel)
		needClear = true
		drawUI()
		return Result{}, false, nil
	}

	drawUI()
	scheduleLiveReload()

	for {
		select {
		case <-winchCh:
			needClear = true
			drawUI()

		case <-reloadTimer.C:
			if app.previewOff {
				_ = ApplyOff(app.repoRoot)
			} else {
				_ = ApplySelection(app.repoRoot, AllEffects[app.effectIdx], AllShapes[app.shapeIdx], app.savedState.OverrideColors)
			}
			signaled := ReloadGhostty()
			if signaled > 0 != app.ghosttyConnected {
				app.ghosttyConnected = signaled > 0
				drawUI()
			}
			resetTimer(stepTimer, 120*time.Millisecond)

		case <-stepTimer.C:
			ansiSeq, nextWait := app.stage.StepNext(currentOrigin)
			if ansiSeq != "" {
				_, _ = io.WriteString(os.Stdout, ShapeDECSCUSR(AllShapes[app.shapeIdx], app.previewOff)+ansiSeq)
			}
			resetTimer(stepTimer, nextWait)

		case rawKey, ok := <-keyCh:
			if !ok {
				_ = RestoreState(app.repoRoot, app.savedState)
				ReloadGhostty()
				return Result{Reverted: true}, nil
			}

			// Handle [y/N] confirmation prompt when saving!
			if app.confirmMode != confirmNone {
				exitAfter := app.confirmMode == confirmSaveExit
				switch rawKey {
				case "\x03": // Ctrl+C still reverts & exits immediately
					_ = RestoreState(app.repoRoot, app.savedState)
					ReloadGhostty()
					exitDECSCUSR = ShapeDECSCUSR(AllShapes[FindShapeIndex(string(app.savedState.Shape))], app.savedState.Off)
					return Result{Reverted: true}, nil
				case "\x1b", "q", "Q": // Esc / q cancels the prompt and returns to browsing
					app.confirmMode = confirmNone
					needClear = true
					drawUI()
				case "y", "Y": // Yes: force Ghostty cursor-color over app/tool OSC 12 colors
					res, done, err := commitSave(true, exitAfter)
					if err != nil {
						return Result{}, err
					}
					if done {
						return res, nil
					}
				case "n", "N", "\r", "\n": // No (default): respect app/tool OSC 12 colors
					res, done, err := commitSave(false, exitAfter)
					if err != nil {
						return Result{}, err
					}
					if done {
						return res, nil
					}
				}
				continue
			}

			switch rawKey {
			case "\x03", "q", "Q", "\x1b": // Revert to savedState & quit!
				_ = RestoreState(app.repoRoot, app.savedState)
				ReloadGhostty()
				exitDECSCUSR = ShapeDECSCUSR(AllShapes[FindShapeIndex(string(app.savedState.Shape))], app.savedState.Off)
				return Result{Reverted: true}, nil

			case "\t", "\x1b[Z", "\x1b[C", "\x1b[D", "h", "l", "H", "L":
				if app.focus == FocusShapes {
					app.focus = FocusEffects
				} else {
					app.focus = FocusShapes
				}
				drawUI()

			case "\x1b[A", "k", "K":
				app.previewOff = false
				app.statusNotice = ""
				if app.focus == FocusShapes {
					app.shapeIdx = (app.shapeIdx - 1 + len(AllShapes)) % len(AllShapes)
				} else {
					app.effectIdx = (app.effectIdx - 1 + len(AllEffects)) % len(AllEffects)
				}
				app.stage.StageForDemo(AllEffects[app.effectIdx].InitialDemo, AllShapes[app.shapeIdx])
				drawUI()
				scheduleLiveReload()

			case "\x1b[B", "j", "J":
				app.previewOff = false
				app.statusNotice = ""
				if app.focus == FocusShapes {
					app.shapeIdx = (app.shapeIdx + 1) % len(AllShapes)
				} else {
					app.effectIdx = (app.effectIdx + 1) % len(AllEffects)
				}
				app.stage.StageForDemo(AllEffects[app.effectIdx].InitialDemo, AllShapes[app.shapeIdx])
				drawUI()
				scheduleLiveReload()

			case " ": // Manual horizontal jump
				app.stage.Paused = false
				app.stage.StageForDemo(DemoJump, AllShapes[app.shapeIdx])
				drawUI()
				resetTimer(stepTimer, 45*time.Millisecond)

			case "w", "W": // Manual Opt+Backspace word-delete
				app.stage.Paused = false
				app.stage.StageForDemo(DemoWordDel, AllShapes[app.shapeIdx])
				drawUI()
				resetTimer(stepTimer, 45*time.Millisecond)

			case "v", "V": // Manual diagonal line jump
				app.stage.Paused = false
				app.stage.StageForDemo(DemoLineJump, AllShapes[app.shapeIdx])
				drawUI()
				resetTimer(stepTimer, 45*time.Millisecond)

			case "p", "P": // Pause / resume motion to inspect pure idle shader
				app.stage.Paused = !app.stage.Paused
				if app.stage.Paused {
					app.stage.StageForDemo(DemoIdle, AllShapes[app.shapeIdx])
				} else {
					resetTimer(stepTimer, 100*time.Millisecond)
				}
				drawUI()

			case "o", "O": // Preview stock native cursor live
				app.previewOff = !app.previewOff
				app.statusNotice = ""
				drawUI()
				scheduleLiveReload()

			case "a", "A": // Save current selection as baseline without closing
				if app.previewOff {
					_ = ApplyOff(app.repoRoot)
					app.savedState.Off = true
					exitDECSCUSR = ShapeDECSCUSR(AllShapes[0], true)
					app.statusNotice = "Saved: off (stock Ghostty cursor)"
					ReloadGhostty()
					drawUI()
				} else {
					app.confirmMode = confirmSaveStay
					needClear = true
					drawUI()
				}

			case "\r", "\n": // Ask tool-color override [y/N], then save & exit!
				if app.previewOff {
					if err := ApplyOff(app.repoRoot); err != nil {
						return Result{}, err
					}
					ReloadGhostty()
					exitDECSCUSR = ShapeDECSCUSR(AllShapes[0], true)
					return Result{Saved: true, TurnedOff: true}, nil
				}
				app.confirmMode = confirmSaveExit
				needClear = true
				drawUI()
			}
		}
	}
}
