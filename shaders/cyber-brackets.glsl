#ifndef SLASH_SWEEP_MODE
#define SLASH_SWEEP_MODE 0
#endif
#ifndef BRACKETS_SHAPE_MODE
#define BRACKETS_SHAPE_MODE 0
#endif

// Sci-Fi Tactical HUD Effect — Converging Target-Lock Reticle + Cardinal Rangefinder Ticks + Holo Scanbeam
// Orthogonal across all 9 cursor shapes (never vanishes; frames bar, slash, block, underscore, hollow, and brackets).

const float GLIDE_DURATION  = 0.075;
const float LOCK_DURATION   = 0.30;
const float STRIKE_IN_TIME  = 0.12;
const float HOLD_AFTER_MOVE = 0.38;
const float CYCLE_PERIOD    = 1.15;
const float MAX_GLIDE_CELLS = 24.0;

float easeOutCubic(float x) {
    float inv = 1.0 - x;
    return 1.0 - inv * inv * inv;
}

float easeOutQuart(float x) {
    float inv = 1.0 - x;
    float inv2 = inv * inv;
    return 1.0 - inv2 * inv2;
}

float easeInOutCubic(float x) {
    return (x < 0.5) ? (4.0 * x * x * x) : (1.0 - pow(-2.0 * x + 2.0, 3.0) * 0.5);
}

float estimateCellHeight(vec4 c) {
    return (iCurrentCursorStyle == CURSORSTYLE_UNDERLINE) ? max(c.z * 2.15, 18.0) : max(c.w, 16.0);
}

float estimateCellWidth(vec4 c) {
    return (iCurrentCursorStyle == CURSORSTYLE_BAR) ? max(c.w * 0.48, 8.0) : max(c.z, 7.0);
}

float sdBox(vec2 p, vec2 b) {
    vec2 d = abs(p) - b;
    return length(max(d, 0.0)) + min(max(d.x, d.y), 0.0);
}

float sdLockBox(vec2 p, vec2 b, float r) {
    float rad = min(r, min(b.x, b.y));
    vec2 q = abs(p) - b + vec2(rad);
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - rad;
}

float cursorShapeSDF(vec2 p, vec2 halfSize) {
#if BRACKETS_SHAPE_MODE == 1
    vec2 b = max(halfSize, vec2(0.8));
    float th = min(0.85, min(b.x, b.y));
    float armX = clamp(b.x * 0.44, th, 4.5);
    float armY = clamp(b.y * 0.34, th, 6.0);
    vec2 q = abs(p);
    float dH = sdBox(q - vec2(b.x - armX * 0.5, b.y - th), vec2(armX * 0.5, th));
    float dV = sdBox(q - vec2(b.x - th, b.y - armY * 0.5), vec2(th, armY * 0.5));
    return min(dH, dV);
#else
    float dBox = sdBox(p, halfSize);
    if (iCurrentCursorStyle == CURSORSTYLE_BLOCK_HOLLOW) {
        return abs(dBox + 0.75) - 0.75;
    }
    return dBox;
#endif
}

// External 4-corner tactical lock reticle + 4 cardinal crosshair ticks around hudHalf
float hudReticleSDF(vec2 p, vec2 hudHalf, float expandPx) {
    vec2 q = abs(p);
    vec2 cornerB = hudHalf + vec2(expandPx);
    float th = 0.75;
    float armX = clamp(cornerB.x * 0.36, 2.2, 5.0);
    float armY = clamp(cornerB.y * 0.32, 2.2, 5.5);

    // 4 outer lock-on corner calipers
    float dCornerH = sdBox(q - vec2(cornerB.x - armX * 0.5, cornerB.y - th), vec2(armX * 0.5, th));
    float dCornerV = sdBox(q - vec2(cornerB.x - th, cornerB.y - armY * 0.5), vec2(th, armY * 0.5));
    float dCorners = min(dCornerH, dCornerV);

    // 4 cardinal rangefinder ticks (top, bottom, left, right midpoints)
    float tickLen = 2.6;
    float tickGap = expandPx + 1.6;
    float dTickLR = sdBox(q - vec2(hudHalf.x + tickGap + tickLen * 0.5, 0.0), vec2(tickLen * 0.5, 0.65));
    float dTickTB = sdBox(q - vec2(0.0, hudHalf.y + tickGap + tickLen * 0.5), vec2(0.65, tickLen * 0.5));

    return min(dCorners, min(dTickLR, dTickTB));
}

vec2 bottomUpSlashBounds(float dt) {
    if (dt < STRIKE_IN_TIME) {
        return vec2(0.0, mix(0.35, 1.0, easeOutQuart(clamp(dt / STRIKE_IN_TIME, 0.0, 1.0))));
    }
    if (dt <= HOLD_AFTER_MOVE) {
        return vec2(0.0, 1.0);
    }
    float p = fract((dt - HOLD_AFTER_MOVE) / CYCLE_PERIOD);
    if (p < 0.34) return vec2(easeInOutCubic(p / 0.34), 1.0);
    if (p < 0.44) return vec2(0.92, 1.0);
    if (p < 0.76) return vec2(0.0, max(0.08, easeOutCubic((p - 0.44) / 0.32)));
    return vec2(0.0, 1.0);
}

vec3 resolveCursorColor() {
    if (distance(iCurrentCursorColor.rgb, iBackgroundColor) > 0.05) {
        return iCurrentCursorColor.rgb;
    }
    if (distance(iCursorColor, iBackgroundColor) > 0.05) {
        return iCursorColor;
    }
    return iForegroundColor;
}

vec3 shadeBlockText(vec3 fillRgb, vec3 glyphSample, vec4 box) {
    vec2 uv1 = clamp(vec2(box.x + 1.0, box.y - 1.0) / iResolution.xy, vec2(0.0), vec2(1.0));
    vec2 uv2 = clamp(vec2(box.x + box.z - 1.0, box.y - box.w + 1.0) / iResolution.xy, vec2(0.0), vec2(1.0));
    vec3 c1 = texture(iChannel0, uv1).rgb;
    vec3 c2 = texture(iChannel0, uv2).rgb;
    vec3 localBg = (distance(c1, iBackgroundColor) <= distance(c2, iBackgroundColor)) ? c1 : c2;
    float glyphInk = smoothstep(0.05, 0.35, distance(glyphSample, localBg));
    float lum = dot(fillRgb, vec3(0.299, 0.587, 0.114));
    vec3 contrastFg = (lum > 0.48) ? min(localBg, vec3(0.10)) : max(iForegroundColor, vec3(0.92));
    return mix(fillRgb, contrastFg, glyphInk);
}

vec4 renderLockCursor(vec4 tex, vec2 fragCoord, vec4 cur, vec3 cursorRgb) {
    vec2 center = vec2(cur.x + cur.z * 0.5, cur.y - cur.w * 0.5);
    vec2 p = fragCoord - center;
    if (abs(p.x) > cur.z * 0.5 + 2.0 || abs(p.y) > cur.w * 0.5 + 2.0) {
        return tex;
    }
    float s = min(cur.z, cur.w * 0.65);
    float dBody = sdLockBox(p - vec2(0.0, -s * 0.22), vec2(s * 0.38, s * 0.28), s * 0.08);
    float dKey = min(
        length(p - vec2(0.0, -s * 0.18)) - s * 0.065,
        sdLockBox(p - vec2(0.0, -s * 0.27), vec2(s * 0.03, s * 0.09), s * 0.015)
    );
    dBody = max(dBody, -dKey);
    vec2 q = p - vec2(0.0, s * 0.10);
    float r = s * 0.22;
    float th = max(s * 0.055, 0.9);
    float dArch = (q.y >= 0.0) ? (abs(length(q) - r) - th) : max(abs(abs(q.x) - r) - th, -q.y - s * 0.16);
    float alpha = (1.0 - smoothstep(-0.55, 0.55, min(dBody, dArch))) * ((iFocus == 0) ? 0.55 : 1.0);
    return vec4(mix(tex.rgb, cursorRgb, alpha), tex.a);
}

vec4 renderUnfocusedCursor(vec4 tex, vec2 fragCoord, vec4 cur, vec3 cursorRgb) {
    vec2 center = vec2(cur.x + cur.z * 0.5, cur.y - cur.w * 0.5);
    vec2 halfU = cur.zw * 0.5;
    vec2 p = fragCoord - center;
    if (abs(p.x) > halfU.x + 2.0 || abs(p.y) > halfU.y + 2.0) {
        return tex;
    }
#if BRACKETS_SHAPE_MODE == 1
    float d = cursorShapeSDF(p, halfU);
#else
    float dBox = sdBox(p, halfU);
    float d = (iCurrentCursorStyle == CURSORSTYLE_BLOCK || iCurrentCursorStyle == CURSORSTYLE_BLOCK_HOLLOW)
        ? (abs(dBox + 0.75) - 0.75)
        : dBox;
#endif
    float alpha = (1.0 - smoothstep(-0.5, 0.5, d)) * 0.55;
    return vec4(mix(tex.rgb, cursorRgb, alpha), tex.a);
}

void mainImage(out vec4 fragColor, in vec2 fragCoord) {
    vec2 uv = fragCoord.xy / iResolution.xy;
    vec4 tex = texture(iChannel0, uv);
    fragColor = tex;

    if (iCursorVisible == 0) {
        return;
    }

    vec4 cur = iCurrentCursor;
    if (cur.z <= 0.0 || cur.w <= 0.0) {
        return;
    }

    vec3 cursorRgb = resolveCursorColor();

    if (iCurrentCursorStyle == CURSORSTYLE_LOCK) {
        fragColor = renderLockCursor(tex, fragCoord, cur, cursorRgb);
        return;
    }

    if (iFocus == 0) {
        fragColor = renderUnfocusedCursor(tex, fragCoord, cur, cursorRgb);
        return;
    }

    float dt = max(0.0, iTime - iTimeCursorChange);
    float cellH = estimateCellHeight(cur);
    float cellW = estimateCellWidth(cur);

    vec4 prev = iPreviousCursor;
    if (prev.z <= 0.0 || prev.w <= 0.0 || distance(prev.xy, cur.xy) > cellH * MAX_GLIDE_CELLS) {
        prev = cur;
    }

    float glideT = easeOutCubic(clamp(dt / GLIDE_DURATION, 0.0, 1.0));
    vec4 box = mix(prev, cur, glideT);

    vec2 baseCenter = vec2(box.x + box.z * 0.5, box.y - box.w * 0.5);
    vec2 fullHalf = box.zw * 0.5;
    vec2 shapeCenter = baseCenter;
    vec2 halfSize = fullHalf;

#if SLASH_SWEEP_MODE == 1
    vec2 bounds = bottomUpSlashBounds(dt);
    float segBot = (box.y - box.w) + box.w * bounds.x;
    float segTop = (box.y - box.w) + box.w * bounds.y;
    shapeCenter = vec2(baseCenter.x, 0.5 * (segTop + segBot));
    halfSize = vec2(fullHalf.x, 0.5 * max(segTop - segBot, 0.5));
#endif

    // Ensure HUD reticle frames narrow shapes (bar / underscore) with generous tactical clearance
    vec2 hudHalf = max(fullHalf, vec2(max(cellW * 0.36, 3.5), max(cellH * 0.24, 4.5)));

    // Converge from +11.5px -> +2.6px on every move, then gently breathe in tactical standby
    float lockSnap = 1.0 - easeOutQuart(clamp(dt / LOCK_DURATION, 0.0, 1.0));
    float idleBreath = 0.65 * (0.5 + 0.5 * sin(iTime * 4.2));
    float expandPx = 2.6 + 8.8 * lockSnap + idleBreath * (1.0 - lockSnap);

    vec2 pBase = fragCoord - baseCenter;
    float maxBoundX = hudHalf.x + expandPx + 7.0;
    float maxBoundY = hudHalf.y + expandPx + 7.0;
    if (abs(pBase.x) > maxBoundX || abs(pBase.y) > maxBoundY) {
        return;
    }

    // Crisp tactical cyan-electric accent blended with user's cursor color
    vec3 hudAccent = mix(cursorRgb, vec3(0.38, 0.94, 1.0), 0.58);
    vec3 hudHot    = mix(hudAccent, vec3(1.0), 0.55);

    vec3 outRgb = tex.rgb;

    // 1. Continuous holographic CRT scanlines + sweeping horizontal laser scan bar
    float crtScan = 0.5 + 0.5 * sin(fragCoord.y * 2.2 - iTime * 11.0);
    float sweepPhase = fract(iTime * 0.78);
    float laserY = mix(-hudHalf.y - 2.0, hudHalf.y + 2.0, 1.0 - sweepPhase);
    float laserBeam = exp(-abs(pBase.y - laserY) * 0.95);

    float hudInteriorD = sdBox(pBase, hudHalf + vec2(expandPx));
    if (hudInteriorD < 0.0) {
        float fieldAlpha = (0.05 + 0.06 * crtScan + 0.18 * laserBeam) * (0.65 + 0.35 * lockSnap);
        outRgb = mix(outRgb, hudAccent, clamp(fieldAlpha, 0.0, 0.35));
    }

    // 2. External tactical lock-on corner calipers + 4 cardinal rangefinder ticks
    float dReticle = hudReticleSDF(pBase, hudHalf, expandPx);
    float reticleGlow = exp(-max(dReticle, 0.0) * 0.85) * (0.22 + 0.28 * lockSnap);
    outRgb += hudAccent * reticleGlow * (1.0 - 0.35 * outRgb);

    float reticleAlpha = (1.0 - smoothstep(-0.55, 0.55, dReticle)) * mix(0.68, 0.98, lockSnap);
    if (reticleAlpha > 0.001) {
        vec3 retColor = mix(hudAccent, hudHot, 0.45 * lockSnap + 0.35 * laserBeam);
        outRgb = mix(outRgb, retColor, reticleAlpha);
    }

    // 3. Core orthogonal cursor shape (100% solid & crisp, with laser highlight + readable block text)
    vec2 pShape = fragCoord - shapeCenter;
    float dShape = cursorShapeSDF(pShape, halfSize);
    if (dShape > 0.0) {
        float coreHalo = exp(-dShape * 0.65) * (0.24 + 0.22 * lockSnap + 0.14 * laserBeam);
        outRgb += hudAccent * coreHalo * (1.0 - 0.35 * outRgb);
    }

    float shapeAlpha = 1.0 - smoothstep(-0.55, 0.55, dShape);
    if (shapeAlpha > 0.001) {
        vec3 fillRgb = mix(cursorRgb, hudHot, 0.30 * lockSnap + 0.25 * laserBeam + 0.08 * crtScan);
        if (iCurrentCursorStyle == CURSORSTYLE_BLOCK && BRACKETS_SHAPE_MODE == 0) {
            fillRgb = shadeBlockText(fillRgb, tex.rgb, cur);
        }
        outRgb = mix(outRgb, fillRgb, shapeAlpha);
    }

    fragColor = vec4(clamp(outRgb, 0.0, 1.0), tex.a);
}
