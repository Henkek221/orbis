// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { clamp, color, curve, font } from "./theme";

// Kinetic type in the style of current product launch films: words arrive one
// by one on the beat, out of a blur, centred; one accent per phrase.
export type Word = { t: string; accent?: boolean; light?: boolean };

const ACCENT = "linear-gradient(90deg, #5ac8fa 0%, #0a84ff 45%, #bf5af2 100%)";

export const Words: React.FC<{
  at: number;
  until: number;
  lines: Word[][];
  step: number; // frames between words, normally one beat or half a beat
  size?: number;
  top?: number; // omitted: vertically centred
  shadow?: boolean;
}> = ({ at, until, lines, step, size = 120, top, shadow = true }) => {
  const frame = useCurrentFrame();
  if (frame < at || frame >= until) return null;
  const out = interpolate(frame, [until - 6, until], [0, 1], clamp);
  let n = 0;
  return (
    <AbsoluteFill
      style={{
        alignItems: "center",
        justifyContent: top === undefined ? "center" : "flex-start",
        paddingTop: top ?? 0,
        opacity: 1 - out,
        filter: out > 0 ? `blur(${out * 14}px)` : undefined,
      }}
    >
      {lines.map((line, li) => (
        <div key={li} style={{ display: "flex", gap: (line[0]?.light ? 0.5 : 1) * size * 0.26, lineHeight: 1.08, marginTop: line[0]?.light ? size * 0.12 : 0 }}>
          {line.map((w, wi) => {
            const start = at + n++ * step;
            const p = interpolate(frame, [start, start + 9], [0, 1], { ...clamp, easing: curve.land });
            return (
              <span
                key={wi}
                style={{
                  fontFamily: font.display,
                  fontSize: w.light ? size * 0.5 : size,
                  fontWeight: w.light ? 500 : 650,
                  letterSpacing: "-0.035em",
                  color: w.light ? "#c7c7cc" : color.text,
                  opacity: p,
                  filter: p < 1 ? `blur(${(1 - p) * 18}px)` : undefined,
                  translate: `0px ${(1 - p) * 36}px`,
                  textShadow: shadow && !w.accent ? "0 4px 48px rgba(0,0,0,0.8)" : undefined,
                  ...(w.accent
                    ? { backgroundImage: ACCENT, WebkitBackgroundClip: "text", backgroundClip: "text", WebkitTextFillColor: "transparent" }
                    : {}),
                }}
              >
                {w.t}
              </span>
            );
          })}
        </div>
      ))}
    </AbsoluteFill>
  );
};

export const w = (s: string, accent = false): Word[] => s.split(" ").map((t) => ({ t, accent }));

// A slow, soft light behind type-only moments, like a studio rim light.
export const Glow: React.FC<{ from: number; until: number; hue?: "blue" | "violet" }> = ({ from, until, hue = "blue" }) => {
  const frame = useCurrentFrame();
  if (frame < from || frame >= until) return null;
  const t = (frame - from) / Math.max(until - from, 1);
  const a = interpolate(frame, [from, from + 10, until - 8, until], [0, 1, 1, 0], clamp);
  const c1 = hue === "blue" ? "10,132,255" : "191,90,242";
  const c2 = hue === "blue" ? "90,200,250" : "10,132,255";
  return (
    <AbsoluteFill
      style={{
        opacity: a,
        background: `radial-gradient(ellipse 60% 45% at ${38 + t * 24}% ${70 - t * 10}%, rgba(${c1},0.42) 0%, rgba(${c1},0) 70%),
                     radial-gradient(ellipse 40% 35% at ${70 - t * 20}% ${30 + t * 8}%, rgba(${c2},0.28) 0%, rgba(${c2},0) 70%)`,
      }}
    />
  );
};

// End card: wordmark, one line, the link as a button.
export const EndCard: React.FC<{ at: number; step: number }> = ({ at, step }) => {
  const frame = useCurrentFrame();
  if (frame < at) return null;
  const p = (k: number) => interpolate(frame, [at + k * step, at + k * step + 10], [0, 1], { ...clamp, easing: curve.land });
  const rise = (k: number) => ({ opacity: p(k), translate: `0px ${(1 - p(k)) * 30}px`, filter: p(k) < 1 ? `blur(${(1 - p(k)) * 14}px)` : undefined });
  return (
    <AbsoluteFill style={{ justifyContent: "center", alignItems: "center" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 34, ...rise(0) }}>
        <Mark size={150} />
        <span style={{ fontFamily: font.display, fontWeight: 700, fontSize: 176, letterSpacing: "-0.045em", color: color.text }}>Orbis</span>
      </div>
      <div style={{ marginTop: 26, fontFamily: font.display, fontWeight: 500, fontSize: 60, color: "#d1d1d6", letterSpacing: "-0.02em", ...rise(1) }}>
        Turn real places into game-ready 3D.
      </div>
      <div
        style={{
          marginTop: 56,
          fontFamily: font.display,
          fontWeight: 600,
          fontSize: 46,
          color: "#fff",
          background: color.blue,
          padding: "22px 48px",
          borderRadius: 999,
          boxShadow: "0 18px 60px rgba(10,132,255,0.45)",
          ...rise(2),
        }}
      >
        github.com/Henkek221/orbis
      </div>
      <div style={{ marginTop: 34, fontFamily: font.display, fontSize: 36, color: color.muted, ...rise(3) }}>GLB · USDZ · Gaussian splats · free on macOS</div>
    </AbsoluteFill>
  );
};

// The Orbis mark: a sphere with one orbit, drawn, not an emoji.
export const Mark: React.FC<{ size: number }> = ({ size }) => (
  <svg width={size} height={size} viewBox="0 0 100 100">
    <defs>
      <linearGradient id="orb" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stopColor="#5ac8fa" />
        <stop offset="0.5" stopColor="#0a84ff" />
        <stop offset="1" stopColor="#bf5af2" />
      </linearGradient>
    </defs>
    <circle cx="50" cy="50" r="30" fill="url(#orb)" />
    <ellipse cx="50" cy="50" rx="46" ry="15" fill="none" stroke="#f5f5f7" strokeWidth="4" transform="rotate(-24 50 50)" />
  </svg>
);
