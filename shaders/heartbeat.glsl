#ifndef SLASH_SWEEP_MODE
#define SLASH_SWEEP_MODE 0
#endif
#ifndef BRACKETS_SHAPE_MODE
#define BRACKETS_SHAPE_MODE 0
#endif

// Heartbeat Effect (Steady Cursor Geometry + Subtle Twin-Beat "lub-dub" Edge Lighting)
// Orthogonal across all cursor shapes (slash, bar, block, slash-block, underscore, hollow, brackets).

const float GLIDE_DURATION  = 0.08;
const float STRIKE_IN_TIME  = 0.12;
const float HOLD_AFTER_MOVE = 0.30;
const float BEAT_PERIOD     = 1.25;
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
    float p = fract((dt - HOLD_AFTER_MOVE) / BEAT_PERIOD);
    if (p < 0.44) return vec2(0.0, 1.0);
    if (p < 0.72) return vec2(easeInOutCubic((p - 0.44) / 0.28), 1.0);
    if (p < 0.78) return vec2(1.0, 1.0);
    return vec2(0.0, easeOutCubic((p - 0.78) / 0.22));
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

// Steady geometry — returns vec2(bodyOpacity, pulseLight)
vec2 heartbeatCurve(float dt) {
    if (dt <= HOLD_AFTER_MOVE) {
        return vec2(1.0, exp(-dt * 12.0) * 0.45);
    }
    float phase = fract((dt - HOLD_AFTER_MOVE) / BEAT_PERIOD);
    float lub = exp(-pow((phase - 0.12) / 0.070, 2.0));
    float dub = exp(-pow((phase - 0.31) / 0.060, 2.0));
    float pulse = clamp(lub * 0.65 + dub * 1.0, 0.0, 1.0);
    float opacity = mix(0.78, 1.0, pulse);
    return vec2(opacity, pulse);
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

    float cellH = estimateCellHeight(cur);
    float dt = max(0.0, iTime - iTimeCursorChange);
    vec2 hb = heartbeatCurve(dt);
    float bodyOpacity = hb.x;
    float pulse = hb.y;

    vec4 prev = iPreviousCursor;
    if (prev.z <= 0.0 || prev.w <= 0.0 || distance(prev.xy, cur.xy) > cellH * MAX_GLIDE_CELLS) {
        prev = cur;
    }

    vec4 box = mix(prev, cur, easeOutCubic(clamp(dt / GLIDE_DURATION, 0.0, 1.0)));
    vec2 center;
    vec2 halfSize;
#if SLASH_SWEEP_MODE == 1
    vec2 bounds = bottomUpSlashBounds(dt);
    if (bounds.y - bounds.x <= 0.003) {
        return;
    }
    float segBot = (box.y - box.w) + box.w * bounds.x;
    float segTop = (box.y - box.w) + box.w * bounds.y;
    center   = vec2(box.x + box.z * 0.5, 0.5 * (segTop + segBot));
    halfSize = vec2(box.z * 0.5, 0.5 * max(segTop - segBot, 0.0));
#else
    center   = vec2(box.x + box.z * 0.5, box.y - box.w * 0.5);
    halfSize = box.zw * 0.5;
#endif

    vec2 p = fragCoord - center;
    if (abs(p.x) > halfSize.x + 8.0 || abs(p.y) > halfSize.y + 8.0) {
        return;
    }

    float d = cursorShapeSDF(p, halfSize);
    vec3 outRgb = tex.rgb;

    if (d > 0.0) {
        float aura = exp(-d * 0.72) * (0.03 + 0.22 * pulse);
        outRgb += cursorRgb * aura * (1.0 - 0.4 * outRgb);
    }

    float alpha = (1.0 - smoothstep(-0.5, 0.5, d)) * bodyOpacity;
    if (alpha > 0.0) {
        vec3 fillRgb = mix(cursorRgb, vec3(1.0), 0.22 * pulse);
        if (iCurrentCursorStyle == CURSORSTYLE_BLOCK && BRACKETS_SHAPE_MODE == 0) {
            fillRgb = shadeBlockText(fillRgb, tex.rgb, cur);
        }
        outRgb = mix(outRgb, fillRgb, alpha);
    }

    fragColor = vec4(clamp(outRgb, 0.0, 1.0), tex.a);
}
