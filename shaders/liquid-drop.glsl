#ifndef SLASH_SWEEP_MODE
#define SLASH_SWEEP_MODE 0
#endif
#ifndef BRACKETS_SHAPE_MODE
#define BRACKETS_SHAPE_MODE 0
#endif

// Liquid Ferrofluid / Mercury Effect (Continuous Meniscus Ribbon + Lens Refraction + Slosh)
// Orthogonal across all cursor shapes (slash, bar, block, slash-block, underscore, hollow, brackets).

const float HEAD_DURATION    = 0.11;
const float STREAM_DURATION  = 0.31;
const float STRIKE_IN_TIME   = 0.12;
const float HOLD_AFTER_MOVE  = 0.44;
const float BLINK_HALF_CYCLE = 0.55;
const float CYCLE_PERIOD     = 1.15;
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

float sdRoundedBox(vec2 p, vec2 b, float r) {
    float rad = min(r, min(b.x, b.y));
    vec2 q = abs(p) - b + vec2(rad);
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - rad;
}

float smin(float a, float b, float k) {
    if (k <= 0.001) {
        return min(a, b);
    }
    float h = max(k - abs(a - b), 0.0) / k;
    return min(a, b) - h * h * k * 0.25;
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
    if (iCurrentCursorStyle == CURSORSTYLE_BLOCK_HOLLOW) {
        float dBox = sdRoundedBox(p, halfSize, 1.0);
        return abs(dBox + 0.75) - 0.75;
    }
    if (iCurrentCursorStyle == CURSORSTYLE_BAR || iCurrentCursorStyle == CURSORSTYLE_UNDERLINE) {
        return sdRoundedBox(p, halfSize, min(halfSize.x, halfSize.y));
    }
    return sdRoundedBox(p, halfSize, 1.4);
#endif
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

float vscodeExpandScale(float dt) {
    if (dt <= HOLD_AFTER_MOVE) {
        return 1.0;
    }
    float t = dt - HOLD_AFTER_MOVE;
    float p = mod(t / BLINK_HALF_CYCLE, 2.0);
    float u = (p <= 1.0) ? p : (2.0 - p);
    return 1.0 - smoothstep(0.16, 0.84, u);
}

vec4 renderLockCursor(vec4 tex, vec2 fragCoord, vec4 cur, vec3 cursorRgb) {
    vec2 center = vec2(cur.x + cur.z * 0.5, cur.y - cur.w * 0.5);
    vec2 p = fragCoord - center;
    if (abs(p.x) > cur.z * 0.5 + 2.0 || abs(p.y) > cur.w * 0.5 + 2.0) {
        return tex;
    }
    float s = min(cur.z, cur.w * 0.65);
    float dBody = sdRoundedBox(p - vec2(0.0, -s * 0.22), vec2(s * 0.38, s * 0.28), s * 0.08);
    float dKey = min(
        length(p - vec2(0.0, -s * 0.18)) - s * 0.065,
        sdRoundedBox(p - vec2(0.0, -s * 0.27), vec2(s * 0.03, s * 0.09), s * 0.015)
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

// Evaluates vec2(combinedLiquidSDF, cursorHeadSDF)
vec2 evalLiquidSDF(
    vec2 pt,
    vec2 headC,
    vec2 halfSize,
    float dt,
    float sloshAmp,
    bool hasStream,
    vec2 tailPos,
    vec2 seg,
    float segLen,
    float streamProgress,
    float cellH
) {
    vec2 relP = pt - headC;
    if (sloshAmp > 0.005) {
        if (iCurrentCursorStyle == CURSORSTYLE_UNDERLINE) {
            relP.y += sin(relP.x * 0.42 - dt * 26.0) * sloshAmp * 0.55;
        } else {
            relP.x += sin(relP.y * 0.36 - dt * 26.0) * sloshAmp;
        }
    }
    float dCursor = cursorShapeSDF(relP, halfSize);

    if (!hasStream) {
        return vec2(dCursor, dCursor);
    }

    vec2 dir = seg / segLen;
    vec2 nrm = vec2(-dir.y, dir.x);
    float h = clamp(dot(pt - tailPos, dir) / segLen, 0.0, 1.0);
    float arch = sin(h * 3.14159265);
    float remain = 1.0 - streamProgress;

    float horizFactor = abs(dir.x);
    float sagY   = -arch * clamp(segLen * 0.055, 0.0, cellH * 0.24) * remain * horizFactor;
    float ripple = arch * sin(h * 8.0 - dt * 28.0) * clamp(segLen * 0.022, 0.0, 1.8) * remain;

    vec2 curvePt = tailPos + seg * h + vec2(0.0, sagY) + nrm * ripple;

    float tipTaper   = smoothstep(0.0, 0.16, h) * pow(h, 0.52);
    float midNeck    = 1.0 - 0.30 * arch * streamProgress;
    float baseRadius = clamp(cellH * 0.10, 1.5, 3.0) * (1.0 - 0.70 * streamProgress * streamProgress) * midNeck;
    float flareSpan  = max(halfSize.y, halfSize.x * 0.45);
    float headFlare  = pow(h, 4.2) * (flareSpan * 0.38) * remain;
    float rStream    = tipTaper * baseRadius + headFlare;

    float dStream = length(pt - curvePt) - rStream;
    bool isWireShape = (BRACKETS_SHAPE_MODE == 1) || (iCurrentCursorStyle == CURSORSTYLE_BLOCK_HOLLOW);
    float meniscusK = cellH * (isWireShape ? 0.14 : 0.26) * remain;

    return vec2(smin(dCursor, dStream, meniscusK), dCursor);
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

    vec2 prevCenter = vec2(prev.x + prev.z * 0.5, prev.y - prev.w * 0.5);
    vec2 curCenter  = vec2(cur.x + cur.z * 0.5, cur.y - cur.w * 0.5);
    float jumpDist  = distance(prevCenter, curCenter);

    float headT = easeOutQuart(clamp(dt / HEAD_DURATION, 0.0, 1.0));
    float streamProgress = clamp(dt / STREAM_DURATION, 0.0, 1.0);
    float tailT = pow(streamProgress, 2.15) * (3.0 - 2.0 * streamProgress);

    vec2 headC   = mix(prevCenter, curCenter, headT);
    vec2 tailPos = mix(prevCenter, curCenter, tailT);

    vec2 fullHalf = cur.zw * 0.5;
#if SLASH_SWEEP_MODE == 1
    headC.y = (headC.y - fullHalf.y) + cur.w * 0.5 * (bounds.x + bounds.y);
#endif

    vec2 pad = vec2(cellH * 0.55 + 14.0, cellH * 0.85 + 14.0);
    vec2 minB = min(headC, tailPos) - pad;
    vec2 maxB = max(headC, tailPos) + pad;
    if (fragCoord.x < minB.x || fragCoord.x > maxB.x || fragCoord.y < minB.y || fragCoord.y > maxB.y) {
        return;
    }

    float jumpFactor = clamp(jumpDist / max(cellH * 0.6, 10.0), 0.15, 1.0);
    float sloshEnv   = exp(-dt * 10.5) * jumpFactor;
    float wobble     = sin(dt * 26.0) * sloshEnv;
    bool isThinLine  = (iCurrentCursorStyle == CURSORSTYLE_BAR || iCurrentCursorStyle == CURSORSTYLE_UNDERLINE);
    float sloshAmp   = (isThinLine ? 1.10 : 0.55) * sloshEnv;

    vec2 halfSize;
    if (iCurrentCursorStyle == CURSORSTYLE_UNDERLINE) {
        halfSize = vec2(fullHalf.x * scale * (1.0 + 0.08 * wobble), max(fullHalf.y, cellH * 0.065 * sloshEnv));
    } else if (iCurrentCursorStyle == CURSORSTYLE_BAR) {
        float swellX = mix(fullHalf.x, max(fullHalf.x, cellH * 0.082), sloshEnv * 0.75) * (1.0 - 0.12 * wobble);
        halfSize = vec2(swellX, fullHalf.y * scale * (1.0 + 0.10 * wobble));
    } else {
        halfSize = vec2(fullHalf.x * (1.0 - 0.04 * wobble), fullHalf.y * scale * (1.0 + 0.05 * wobble));
    }

    vec2 seg = headC - tailPos;
    float segLen = length(seg);
    bool hasStream = (dt < STREAM_DURATION) && (jumpDist > 2.5) && (segLen > 1.2);

    vec2 field = evalLiquidSDF(
        fragCoord, headC, halfSize, dt, sloshAmp,
        hasStream, tailPos, seg, segLen, streamProgress, cellH
    );
    float d = field.x;
    float dCursor = field.y;

    if (d > 8.0) {
        return;
    }

    float dX = evalLiquidSDF(
        fragCoord + vec2(1.0, 0.0), headC, halfSize, dt, sloshAmp,
        hasStream, tailPos, seg, segLen, streamProgress, cellH
    ).x;
    float dY = evalLiquidSDF(
        fragCoord + vec2(0.0, 1.0), headC, halfSize, dt, sloshAmp,
        hasStream, tailPos, seg, segLen, streamProgress, cellH
    ).x;
    vec2 grad = vec2(dX - d, dY - d);
    vec2 n2 = grad / max(length(grad), 0.0001);

    float liquidMask = 1.0 - smoothstep(-0.55, 0.55, d);
    float headMask   = 1.0 - smoothstep(-0.55, 0.55, dCursor);

    float motionVis = hasStream ? (1.0 - streamProgress) : sloshEnv;
    float wetGlow = exp(-max(d, 0.0) * 0.55) * 0.20 * motionVis;
    vec3 outRgb = tex.rgb + cursorRgb * wetGlow * (1.0 - 0.4 * tex.rgb);

    if (liquidMask > 0.001) {
        float depth = clamp(-d / max(cellH * 0.15, 2.5), 0.0, 1.0);
        vec2 refrUv = clamp(uv - n2 * (2.2 * (1.0 - 0.5 * depth) * motionVis) / iResolution.xy, vec2(0.0), vec2(1.0));
        vec3 refractedBg = texture(iChannel0, refrUv).rgb;

        vec3 N = normalize(vec3(n2 * (1.0 - 0.65 * depth), 0.52 + 0.48 * depth));
        vec3 L = normalize(vec3(-0.45, 0.68, 0.58));
        float spec    = pow(max(dot(N, L), 0.0), 16.0) * (0.40 + 0.45 * motionVis);
        float fresnel = pow(1.0 - N.z, 2.4) * 0.28;

        float bodyDensity = mix(0.70, 1.0, headMask);
        vec3 fluidRgb = mix(refractedBg, cursorRgb, bodyDensity);
        fluidRgb = mix(fluidRgb, vec3(1.0), clamp(spec + fresnel, 0.0, 0.78));

        if (iCurrentCursorStyle == CURSORSTYLE_BLOCK && BRACKETS_SHAPE_MODE == 0 && headMask > 0.01) {
            vec3 blockRgb = shadeBlockText(mix(cursorRgb, vec3(1.0), clamp(spec * 0.45, 0.0, 0.35)), tex.rgb, cur);
            fluidRgb = mix(fluidRgb, blockRgb, headMask);
        }

        outRgb = mix(outRgb, fluidRgb, liquidMask);
    }

    fragColor = vec4(clamp(outRgb, 0.0, 1.0), tex.a);
}
