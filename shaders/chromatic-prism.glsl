#ifndef SLASH_SWEEP_MODE
#define SLASH_SWEEP_MODE 0
#endif
#ifndef BRACKETS_SHAPE_MODE
#define BRACKETS_SHAPE_MODE 0
#endif

// Chromatic Prism Effect (RGB Dispersion on Movement + Iridescent Shimmer)
// Orthogonal across all cursor shapes (slash, bar, block, slash-block, underscore, hollow, brackets).

const float STRIKE_IN_TIME   = 0.12;
const float HOLD_AFTER_MOVE  = 0.35;
const float BLINK_HALF_CYCLE = 0.50;
const float CYCLE_PERIOD     = 1.15;
const float PRISM_SHIMMER    = 0.38;
const float MAX_GLIDE_CELLS  = 24.0;

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

vec2 bottomUpSlashBounds(float dt) {
    if (dt < STRIKE_IN_TIME) {
        return vec2(0.0, mix(0.35, 1.0, easeOutQuart(clamp(dt / STRIKE_IN_TIME, 0.0, 1.0))));
    }
    if (dt <= HOLD_AFTER_MOVE) {
        return vec2(0.0, 1.0);
    }
    float p = fract((dt - HOLD_AFTER_MOVE) / CYCLE_PERIOD);
    if (p < 0.34) return vec2(easeInOutCubic(p / 0.34), 1.0);
    if (p < 0.44) return vec2(1.0, 1.0);
    if (p < 0.76) return vec2(0.0, easeOutCubic((p - 0.44) / 0.32));
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

float vscodeExpandScale(float dt) {
    if (dt <= HOLD_AFTER_MOVE) {
        return 1.0;
    }
    float t = dt - HOLD_AFTER_MOVE;
    float p = mod(t / BLINK_HALF_CYCLE, 2.0);
    float u = (p <= 1.0) ? p : (2.0 - p);
    return 1.0 - smoothstep(0.18, 0.82, u);
}

float channelMask(vec2 fragCoord, vec4 box, float scale, vec2 bounds) {
#if SLASH_SWEEP_MODE == 1
    float segBot = (box.y - box.w) + box.w * bounds.x;
    float segTop = (box.y - box.w) + box.w * bounds.y;
    vec2 center = vec2(box.x + box.z * 0.5, 0.5 * (segTop + segBot));
    vec2 halfSize = vec2(box.z * 0.5, 0.5 * max(segTop - segBot, 0.0));
#else
    vec2 center = vec2(box.x + box.z * 0.5, box.y - box.w * 0.5);
    vec2 fullHalf = box.zw * 0.5;
    vec2 halfSize = (iCurrentCursorStyle == CURSORSTYLE_UNDERLINE)
        ? vec2(fullHalf.x * scale, fullHalf.y)
        : vec2(fullHalf.x, fullHalf.y * scale);
#endif
    float d = cursorShapeSDF(fragCoord - center, halfSize);
    return 1.0 - smoothstep(-0.6, 0.6, d);
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
#if SLASH_SWEEP_MODE == 1
    vec2 bounds = bottomUpSlashBounds(dt);
    float scale = bounds.y - bounds.x;
#else
    vec2 bounds = vec2(0.0, 1.0);
    float scale = vscodeExpandScale(dt);
#endif
    if (scale <= 0.002) {
        return;
    }

    float cellH = estimateCellHeight(cur);
    vec4 prev = iPreviousCursor;
    if (prev.z <= 0.0 || prev.w <= 0.0 || distance(prev.xy, cur.xy) > cellH * MAX_GLIDE_CELLS) {
        prev = cur;
    }

    vec4 boxR = mix(prev, cur, easeOutCubic(clamp(dt / 0.055, 0.0, 1.0)));
    vec4 boxG = mix(prev, cur, easeOutCubic(clamp(dt / 0.085, 0.0, 1.0)));
    vec4 boxB = mix(prev, cur, easeOutCubic(clamp(dt / 0.125, 0.0, 1.0)));

    vec2 minPt = min(min(boxR.xy - vec2(0.0, boxR.w), boxB.xy - vec2(0.0, boxB.w)), cur.xy - vec2(0.0, cur.w)) - vec2(4.0);
    vec2 maxPt = max(max(boxR.xy + vec2(boxR.z, 0.0), boxB.xy + vec2(boxB.z, 0.0)), cur.xy + vec2(cur.z, 0.0)) + vec2(4.0);
    if (fragCoord.x < minPt.x || fragCoord.x > maxPt.x || fragCoord.y < minPt.y || fragCoord.y > maxPt.y) {
        return;
    }

    float mR = channelMask(fragCoord, boxR, scale, bounds);
    float mG = channelMask(fragCoord, boxG, scale, bounds);
    float mB = channelMask(fragCoord, boxB, scale, bounds);
    float anyMask = max(mR, max(mG, mB));
    if (anyMask <= 0.001) {
        return;
    }

    vec2 gCenter = vec2(boxG.x + boxG.z * 0.5, boxG.y - boxG.w * 0.5);
    vec3 spectral = 0.55 + 0.45 * cos(iTime * 3.2 + (fragCoord.y - gCenter.y) * 0.11 + vec3(0.0, 2.094, 4.188));
    vec3 baseCol = mix(cursorRgb, spectral, PRISM_SHIMMER);

    if (iCurrentCursorStyle == CURSORSTYLE_BLOCK && BRACKETS_SHAPE_MODE == 0) {
        baseCol = shadeBlockText(baseCol, tex.rgb, cur);
    }

    vec3 outRgb = vec3(
        mix(tex.r, baseCol.r, mR),
        mix(tex.g, baseCol.g, mG),
        mix(tex.b, baseCol.b, mB)
    );

    fragColor = vec4(outRgb, tex.a);
}
