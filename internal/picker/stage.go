package picker

import (
	"fmt"
	"strings"
	"time"
)

const (
	ansiCodeBG    = "\x1b[48;2;14;17;24m"
	ansiGutterFG  = "\x1b[38;2;68;76;108m"
	ansiKwFG      = "\x1b[38;2;122;162;247m"
	ansiFnFG      = "\x1b[38;2;125;207;255m"
	ansiTypeFG    = "\x1b[38;2;42;195;222m"
	ansiIdFG      = "\x1b[38;2;192;202;245m"
	ansiPunctFG   = "\x1b[38;2;137;221;255m"
	ansiCommentFG = "\x1b[38;2;86;95;137m"
)

// StageOrigin holds the 1-based terminal screen coordinates of row 0, col 0 inside preview.go.
type StageOrigin struct {
	TopRow  int // 1-based terminal row of editor line 0
	LeftCol int // 1-based terminal column of editor col 0
	Cols    int
	Rows    int
}

func (o StageOrigin) ScreenPos(edRow, edCol int) (int, int) {
	if edCol < 0 {
		edCol = 0
	}
	if edCol >= o.Cols {
		edCol = o.Cols - 1
	}
	if edRow < 0 {
		edRow = 0
	}
	if edRow >= o.Rows {
		edRow = o.Rows - 1
	}
	return o.TopRow + edRow, o.LeftCol + edCol
}

// Editor row 3 has prefix: " 4      blade := shader." (24 columns: indices 0..23)
// Typed suffix is:         "Unsheathe(ctx)"          (14 columns: indices 24..37, end col = 38)
const (
	stagePrefixCol = 24
	stageTypedWord = "Unsheathe(ctx)"
	stageEndCol    = stagePrefixCol + len(stageTypedWord) // 38
)

type stageOp int

const (
	opMoveCursor stageOp = iota // Pure \x1b[r;cH cursor move
	opTypeChar                  // Emit single colored rune at current cursor pos (advances cursor +1)
	opWordDelete                // Jump left + \x1b[14X erase word in-place without extra cursor move
)

type stageStep struct {
	op   stageOp
	row  int
	col  int
	ch   byte
	chFG string
	wait time.Duration
}

// LiveStage drives Ghostty's REAL hardware/GPU cursor across the preview.go buffer
// using surgical 8-16 byte ANSI sequences so libghostty renders the actual GLSL shader!
type LiveStage struct {
	CurRow      int
	CurCol      int
	WordVisible bool
	Paused      bool
	ActiveDemo  DemoKind
	ActiveShape ShapeInfo
	queue       []stageStep
}

func NewLiveStage() *LiveStage {
	return &LiveStage{
		CurRow:      3,
		CurCol:      stageEndCol,
		WordVisible: true,
		ActiveDemo:  DemoJump,
		ActiveShape: AllShapes[0],
	}
}

// RenderEditorLines renders the initial static lines inside preview.go for full-screen redraws.
func (s *LiveStage) RenderEditorLines(cols, rows int) []string {
	lines := make([]string, rows)
	makeRow := func(lineNum int, content string, visContent int) string {
		gutter := fmt.Sprintf(" %d  ", lineNum)
		pad := maxInt(0, cols-4-visContent)
		return ansiCodeBG + ansiGutterFG + gutter + content + ansiCodeBG + strings.Repeat(" ", pad) + ansiReset
	}

	if rows > 0 {
		lines[0] = makeRow(1, ansiKwFG+"package "+ansiIdFG+"cursor", 14)
	}
	if rows > 1 {
		lines[1] = makeRow(2, "", 0)
	}
	if rows > 2 {
		c := ansiKwFG + "func " + ansiFnFG + "render" + ansiPunctFG + "(" + ansiIdFG + "dt " + ansiTypeFG + "float32" + ansiPunctFG + ") {"
		lines[2] = makeRow(3, c, 25)
	}
	if rows > 3 {
		base := "    " + ansiIdFG + "blade " + ansiPunctFG + ":= " + ansiIdFG + "shader" + ansiPunctFG + "."
		vis := 20
		if s.WordVisible {
			base += ansiFnFG + "Unsheathe" + ansiPunctFG + "(" + ansiIdFG + "ctx" + ansiPunctFG + ")"
			vis += len(stageTypedWord)
		}
		lines[3] = makeRow(4, base, vis)
	}
	if rows > 4 {
		c := "    " + ansiKwFG + "return " + ansiFnFG + "Blend" + ansiPunctFG + "(" + ansiIdFG + "blade" + ansiPunctFG + ", " + ansiIdFG + "dt" + ansiPunctFG + ")"
		lines[4] = makeRow(5, c, 27)
	}
	if rows > 5 {
		lines[5] = makeRow(6, ansiPunctFG+"}", 1)
	}
	for r := 6; r < rows; r++ {
		if r == rows-1 && rows >= 8 {
			hint := "// real libghostty GPU cursor stage"
			if len(hint)+4 <= cols {
				lines[r] = makeRow(r+1, ansiCommentFG+hint, len(hint))
				continue
			}
		}
		lines[r] = makeRow(r+1, "", 0)
	}
	return lines
}

// StageForDemo pre-positions the stage state BEFORE SIGUSR2 reloads Ghostty, and queues
// the signature cursor move to fire right AFTER Ghostty loads the new shader.
func (s *LiveStage) StageForDemo(kind DemoKind, shape ShapeInfo) {
	s.ActiveDemo = kind
	s.ActiveShape = shape
	s.queue = s.queue[:0]

	switch kind {
	case DemoWordDel:
		// Pre-stage at end of "Unsheathe(ctx)", then fire Opt+Backspace leftward slash!
		s.WordVisible = true
		s.CurRow = 3
		s.CurCol = stageEndCol
		s.queue = append(s.queue,
			stageStep{op: opWordDelete, row: 3, col: stagePrefixCol, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 4, col: 15, wait: 850 * time.Millisecond},
		)

	case DemoJump:
		// Keep full code visible; pre-stage at shader. (col 24), then leap right to col 38 & left onto 'b' of blade!
		s.WordVisible = true
		s.CurRow = 3
		s.CurCol = stagePrefixCol
		s.queue = append(s.queue,
			stageStep{op: opMoveCursor, row: 3, col: stageEndCol, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 3, col: 8, wait: 850 * time.Millisecond},
		)

	case DemoLineJump:
		s.WordVisible = true
		s.CurRow = 3
		s.CurCol = stageEndCol
		s.queue = append(s.queue,
			stageStep{op: opMoveCursor, row: 4, col: 15, wait: 850 * time.Millisecond}, // onto 'B' of Blend
			stageStep{op: opMoveCursor, row: 3, col: stageEndCol, wait: 850 * time.Millisecond},
		)

	default: // DemoIdle
		s.WordVisible = true
		if shape.NativeStyle == StyleBlock && !shape.BracketsSDF {
			// Park directly on 'B' of Blend (row 4, col 15) so solid block text contrast shines!
			s.CurRow = 4
			s.CurCol = 15
		} else {
			// Park on clean cell after Unsheathe(ctx) so hollow/brackets/bar/slash/underscore are unobstructed
			s.CurRow = 3
			s.CurCol = stageEndCol
		}
		// Hold completely undisturbed (0 bytes emitted during wait!) so idle expand/pulse/sweep plays cleanly!
		s.queue = append(s.queue,
			stageStep{op: opMoveCursor, row: s.CurRow, col: s.CurCol, wait: 2400 * time.Millisecond},
		)
	}
}

// refillLoop continues repeating choreography tailored to the currently highlighted effect & shape.
func (s *LiveStage) refillLoop() {
	switch s.ActiveDemo {
	case DemoWordDel:
		if !s.WordVisible {
			if s.CurRow != 3 || s.CurCol != stagePrefixCol {
				s.queue = append(s.queue, stageStep{op: opMoveCursor, row: 3, col: stagePrefixCol, wait: 350 * time.Millisecond})
			}
			for i := 0; i < len(stageTypedWord); i++ {
				ch := stageTypedWord[i]
				fg := ansiFnFG
				switch {
				case ch == '(' || ch == ')':
					fg = ansiPunctFG
				case i >= 10:
					fg = ansiIdFG
				}
				wait := 32 * time.Millisecond
				if i == len(stageTypedWord)-1 {
					wait = 450 * time.Millisecond
				}
				s.queue = append(s.queue, stageStep{
					op:   opTypeChar,
					row:  3,
					col:  stagePrefixCol + i + 1,
					ch:   ch,
					chFG: fg,
					wait: wait,
				})
			}
		}
		s.queue = append(s.queue,
			stageStep{op: opWordDelete, row: 3, col: stagePrefixCol, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 4, col: 15, wait: 850 * time.Millisecond},
		)

	case DemoIdle:
		s.queue = append(s.queue,
			stageStep{op: opMoveCursor, row: 4, col: 15, wait: 2400 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 3, col: stageEndCol, wait: 2400 * time.Millisecond},
		)

	default: // DemoJump & DemoLineJump
		s.queue = append(s.queue,
			stageStep{op: opMoveCursor, row: 3, col: stageEndCol, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 3, col: 8, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 4, col: 15, wait: 850 * time.Millisecond},
			stageStep{op: opMoveCursor, row: 3, col: stagePrefixCol, wait: 850 * time.Millisecond},
		)
	}
}

// StepNext executes one surgical stage operation and returns (ansiBytesToEmit, delayUntilNextStep).
func (s *LiveStage) StepNext(origin StageOrigin) (string, time.Duration) {
	if s.Paused {
		return "", 500 * time.Millisecond
	}
	if len(s.queue) == 0 {
		s.refillLoop()
	}
	step := s.queue[0]
	s.queue = s.queue[1:]

	switch step.op {
	case opTypeChar:
		var sb strings.Builder
		wantCol := step.col - 1
		if s.CurRow != step.row || s.CurCol != wantCol {
			sr, sc := origin.ScreenPos(step.row, wantCol)
			fmt.Fprintf(&sb, "\x1b[%d;%dH", sr, sc)
		}
		sb.WriteString(ansiCodeBG)
		sb.WriteString(step.chFG)
		sb.WriteByte(step.ch)
		sb.WriteString(ansiReset)
		s.CurRow = step.row
		s.CurCol = step.col
		if step.col >= stageEndCol {
			s.WordVisible = true
		}
		return sb.String(), step.wait

	case opWordDelete:
		// Single atomic jump to (row, stagePrefixCol) + ECH (\x1b[14X) to erase "Unsheathe(ctx)" WITHOUT moving cursor!
		sr, sc := origin.ScreenPos(step.row, step.col)
		eraseN := len(stageTypedWord)
		if step.col+eraseN > origin.Cols {
			eraseN = maxInt(1, origin.Cols-step.col)
		}
		s.CurRow = step.row
		s.CurCol = step.col
		s.WordVisible = false
		return fmt.Sprintf("%s\x1b[%d;%dH\x1b[%dX%s", ansiCodeBG, sr, sc, eraseN, ansiReset), step.wait

	default: // opMoveCursor
		sr, sc := origin.ScreenPos(step.row, step.col)
		s.CurRow = step.row
		s.CurCol = step.col
		return fmt.Sprintf("\x1b[%d;%dH", sr, sc), step.wait
	}
}
