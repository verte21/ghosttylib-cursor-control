package picker

// ShapeID identifies one of the 9 orthogonal cursor geometries.
type ShapeID string

const (
	ShapeSlash         ShapeID = "slash"
	ShapeBar           ShapeID = "bar"
	ShapeBlock         ShapeID = "block"
	ShapeSlashBlock    ShapeID = "slash-block"
	ShapeUnderscore    ShapeID = "underscore"
	ShapeHollow        ShapeID = "hollow"
	ShapeSlashHollow   ShapeID = "slash-hollow"
	ShapeBrackets      ShapeID = "brackets"
	ShapeSlashBrackets ShapeID = "slash-brackets"
)

// NativeStyle mirrors Ghostty's built-in CURSORSTYLE_* enum.
type NativeStyle int

const (
	StyleBar         NativeStyle = 0
	StyleBlock       NativeStyle = 1
	StyleUnderline   NativeStyle = 2
	StyleBlockHollow NativeStyle = 3
)

// ShapeInfo holds display and shader-generation metadata for a cursor shape.
type ShapeInfo struct {
	ID           ShapeID
	Summary      string
	NativeStyle  NativeStyle
	GhosttyStyle string
	SlashSweep   bool
	BracketsSDF  bool
}

// EffectID identifies one of the 10 curated orthogonal cursor shader effects.
type EffectID string

const (
	EffectKatana    EffectID = "katana"
	EffectExpand    EffectID = "expand"
	EffectNeon      EffectID = "neon"
	EffectJelly     EffectID = "jelly"
	EffectPrism     EffectID = "prism"
	EffectHUD       EffectID = "hud"
	EffectRipple    EffectID = "ripple"
	EffectLiquid    EffectID = "liquid"
	EffectHeartbeat EffectID = "heartbeat"
	EffectSmooth    EffectID = "smooth"
)

// DemoKind controls which motion impulse triggers immediately when selecting an item.
type DemoKind int

const (
	DemoIdle     DemoKind = iota // Immediate idle/pulse showcase
	DemoWordDel                  // Horizontal word-delete jump (leftward)
	DemoJump                     // Horizontal multi-cell jump (rightward)
	DemoLineJump                 // Diagonal line-to-line jump
)

// EffectInfo holds display and shader metadata for an effect.
type EffectInfo struct {
	ID          EffectID
	Summary     string
	ShaderFile  string
	InitialDemo DemoKind
}

var AllShapes = []ShapeInfo{
	{
		ID:           ShapeSlash,
		Summary:      "Thin vertical blade sweeping bottom-to-top",
		NativeStyle:  StyleBar,
		GhosttyStyle: "bar",
		SlashSweep:   true,
	},
	{
		ID:           ShapeBar,
		Summary:      "Classic crisp vertical bar on the left cell edge",
		NativeStyle:  StyleBar,
		GhosttyStyle: "bar",
	},
	{
		ID:           ShapeBlock,
		Summary:      "Full-cell solid block with high-contrast text underneath",
		NativeStyle:  StyleBlock,
		GhosttyStyle: "block",
	},
	{
		ID:           ShapeSlashBlock,
		Summary:      "Full-cell solid block sweeping bottom-to-top",
		NativeStyle:  StyleBlock,
		GhosttyStyle: "block",
		SlashSweep:   true,
	},
	{
		ID:           ShapeUnderscore,
		Summary:      "Horizontal underscore along the bottom of the cell",
		NativeStyle:  StyleUnderline,
		GhosttyStyle: "underline",
	},
	{
		ID:           ShapeHollow,
		Summary:      "1.5px hollow wireframe outline box",
		NativeStyle:  StyleBlockHollow,
		GhosttyStyle: "block_hollow",
	},
	{
		ID:           ShapeSlashHollow,
		Summary:      "1.5px hollow outline box sweeping bottom-to-top",
		NativeStyle:  StyleBlockHollow,
		GhosttyStyle: "block_hollow",
		SlashSweep:   true,
	},
	{
		ID:           ShapeBrackets,
		Summary:      "4-corner [ ] caliper reticle around the cell",
		NativeStyle:  StyleBlockHollow,
		GhosttyStyle: "block_hollow",
		BracketsSDF:  true,
	},
	{
		ID:           ShapeSlashBrackets,
		Summary:      "4-corner [ ] caliper reticle sweeping bottom-to-top",
		NativeStyle:  StyleBlockHollow,
		GhosttyStyle: "block_hollow",
		SlashSweep:   true,
		BracketsSDF:  true,
	},
}

var AllEffects = []EffectInfo{
	{
		ID:          EffectKatana,
		Summary:     "Horizontal execution slash on word delete + curved line-jump arc",
		ShaderFile:  "katana-slash.glsl",
		InitialDemo: DemoWordDel,
	},
	{
		ID:          EffectExpand,
		Summary:     "VS Code symmetric center expand + smooth 85ms caret glide",
		ShaderFile:  "vscode-expand.glsl",
		InitialDemo: DemoIdle,
	},
	{
		ID:          EffectNeon,
		Summary:     "Dual-layer HDR neon bloom halo + keystroke spark + smooth glide",
		ShaderFile:  "neon-expand.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectJelly,
		Summary:     "Pixel-clamped spring overshoot + shape-preserving squash & stretch",
		ShaderFile:  "elastic-jelly.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectPrism,
		Summary:     "Staggered R/G/B chromatic dispersion on motion + spectral shimmer",
		ShaderFile:  "chromatic-prism.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectHUD,
		Summary:     "Tactical target-lock reticle + converging rangefinder ticks + holo scanbeam",
		ShaderFile:  "cyber-brackets.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectRipple,
		Summary:     "Keystroke pulse + expanding refractive shockwave ring on jumps",
		ShaderFile:  "sonic-ripple.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectLiquid,
		Summary:     "Continuous ferrofluid meniscus tendril + whip-snap + arrival slosh",
		ShaderFile:  "liquid-drop.glsl",
		InitialDemo: DemoJump,
	},
	{
		ID:          EffectHeartbeat,
		Summary:     "Steady non-expanding cursor + subtle twin-beat rim lighting",
		ShaderFile:  "heartbeat.glsl",
		InitialDemo: DemoIdle,
	},
	{
		ID:          EffectSmooth,
		Summary:     "Silky cosine-wave opacity fade without size change",
		ShaderFile:  "vscode-smooth.glsl",
		InitialDemo: DemoIdle,
	},
}

func FindShapeIndex(name string) int {
	switch name {
	case "sweep", "up", "bottom-up", "bottom-to-top":
		name = string(ShapeSlash)
	case "line", "beam":
		name = string(ShapeBar)
	case "block-slash":
		name = string(ShapeSlashBlock)
	case "underline":
		name = string(ShapeUnderscore)
	case "block_hollow":
		name = string(ShapeHollow)
	case "hollow-slash":
		name = string(ShapeSlashHollow)
	case "calipers", "reticle":
		name = string(ShapeBrackets)
	case "brackets-slash":
		name = string(ShapeSlashBrackets)
	}
	for i, s := range AllShapes {
		if string(s.ID) == name {
			return i
		}
	}
	return 0
}

func FindEffectIndex(name string) int {
	switch name {
	case "katana-down", "katana-up":
		name = string(EffectKatana)
	case "expand-glide", "expand-trail", "top-down", "bottom-up":
		name = string(EffectExpand)
	case "brackets", "recoil":
		name = string(EffectHUD)
	case "phase":
		name = string(EffectSmooth)
	}
	for i, e := range AllEffects {
		if string(e.ID) == name {
			return i
		}
	}
	return 0
}
