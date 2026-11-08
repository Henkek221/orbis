import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { clamp, color, curve, font } from "./theme";

// Real command and real output of the Brandenburger Tor export.
export const COMMAND = 'orbis export --place "Brandenburger Tor"';
const OUTPUT = [
  "area: 200 x 200 m around 52.51628,13.37770",
  "format: R3D v2 (version 828), meshes and splats",
  "tiles: zoom=20 h=0..23 (2160 requests)",
];
const DONE = "90 tiles · 252,091 splats";

// Human typing: uneven gaps, short bursts, a pause before the quoted place.
const GAPS = [3, 2, 2, 3, 2, 4, 2, 2, 3, 2, 2, 2, 3, 2, 2, 6, 2, 3, 2, 2, 2, 3, 2, 4, 2, 2, 3, 2, 2, 2, 3, 2, 2, 2, 3, 2, 2, 3, 2, 2];
export const typedAt = (i: number) => GAPS.slice(0, i).reduce((a, b) => a + b, 0);
export const TYPE_END = typedAt(COMMAND.length);
export const ENTER = TYPE_END + 8;

export const Terminal: React.FC<{ progressStart: number; progressEnd: number; shrinkAt: number }> = ({
  progressStart,
  progressEnd,
  shrinkAt,
}) => {
  const frame = useCurrentFrame();
  let typed = 0;
  while (typed < COMMAND.length && typedAt(typed + 1) <= frame) typed++;
  const progress = interpolate(frame, [progressStart, progressEnd], [0, 1], clamp);
  const bar = 16;
  const filled = Math.round(progress * bar);

  return (
    <AbsoluteFill
      style={{
        // The terminal shrinks into the lower left corner while the city builds.
        scale: interpolate(frame, [shrinkAt, shrinkAt + 24], [1, 0.62], { ...clamp, easing: curve.glide }),
        translate: interpolate(frame, [shrinkAt, shrinkAt + 24], ["0px 0px", "-380px 330px"], { ...clamp, easing: curve.glide }),
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      <div
        style={{
          width: 1560,
          padding: "52px 64px",
          borderRadius: 26,
          background: color.terminal,
          boxShadow: "0 40px 120px rgba(0,0,0,0.6), inset 0 0 0 1px rgba(255,255,255,0.06)",
          fontFamily: font.mono,
          fontSize: 50,
          lineHeight: 1.45,
          color: color.text,
        }}
      >
        <div>
          <span style={{ color: color.green }}>~ </span>
          <span style={{ color: color.muted }}>% </span>
          {COMMAND.slice(0, typed)}
          {frame < ENTER ? <span style={{ opacity: Math.floor(frame / 15) % 2 === 0 ? 1 : 0 }}>▍</span> : null}
        </div>
        {OUTPUT.map((line, i) =>
          frame >= ENTER + 4 + i * 5 && frame < shrinkAt + 12 ? (
            <div key={line} style={{ color: color.muted, fontSize: 38, opacity: interpolate(frame, [shrinkAt, shrinkAt + 12], [1, 0], clamp) }}>
              {line}
            </div>
          ) : null,
        )}
        {frame >= progressStart ? (
          <div style={{ fontSize: 58, color: progress >= 1 ? color.green : color.text }}>
            {"█".repeat(filled)}
            <span style={{ color: "#2c2c2e" }}>{"█".repeat(bar - filled)}</span>
            {`  ${Math.round(progress * 100)}%`}
          </div>
        ) : null}
        {progress >= 1 ? <div style={{ fontSize: 58, color: color.green }}>{DONE}</div> : null}
      </div>
    </AbsoluteFill>
  );
};
