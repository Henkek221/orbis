// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

import React from "react";
import { AbsoluteFill, Audio, Img, Sequence, interpolate, staticFile, useCurrentFrame } from "remotion";
import { City3D } from "./City3D";
import { clamp, curve, FPS } from "./theme";

// A light launch film built on the grammar of dub.co's launch video: white
// grid, black regular-weight type whose newest word arrives grey and settles to
// black, white cards tilted in 3D with soft shadows, a typed link at the end.
// Music: "Smooth Grooves" (Mixkit), 100.04 BPM, bed starts on a beat.

const BPM = 100.04;
const B = (60 / BPM) * FPS;
const b = (n: number) => Math.round(n * B);

const ink = "#0b0b0c";
const grey = "#a1a1a6";
const line = "#e9e9ec";
const blue = "#0a84ff";
const sans = '-apple-system, "SF Pro Display", "Inter", "Helvetica Neue", sans-serif';
const mono = '"SF Mono", ui-monospace, Menlo, monospace';
const cardShadow = "0 1px 2px rgba(0,0,0,0.05), 0 18px 50px rgba(70,60,140,0.10)";

const ease = (f: number, a: number, d: number) => interpolate(f, [a, a + d], [0, 1], { ...clamp, easing: curve.land });

// ---------- background ----------

const Grid: React.FC = () => (
  <AbsoluteFill style={{ background: "#ffffff" }}>
    <AbsoluteFill
      style={{
        backgroundImage: "linear-gradient(rgba(0,0,0,0.045) 1px, transparent 1px), linear-gradient(90deg, rgba(0,0,0,0.045) 1px, transparent 1px)",
        backgroundSize: "90px 90px",
        backgroundPosition: "center center",
        WebkitMaskImage: "radial-gradient(ellipse 70% 65% at 50% 50%, #000 30%, transparent 100%)",
        maskImage: "radial-gradient(ellipse 70% 65% at 50% 50%, #000 30%, transparent 100%)",
      }}
    />
  </AbsoluteFill>
);

// ---------- type ----------

// Words arrive one by one: grey and slightly low, then black on the line.
const Reveal: React.FC<{
  at: number;
  step: number;
  text: string;
  size?: number;
  color?: string;
  weight?: number;
  align?: "center" | "left";
  lines?: string[]; // optional manual line breaks; overrides text
}> = ({ at, step, text, size = 132, color = ink, weight = 500, align = "center", lines }) => {
  const frame = useCurrentFrame();
  const rows = (lines ?? [text]).map((l) => l.split(" "));
  let n = 0;
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: align === "center" ? "center" : "flex-start" }}>
      {rows.map((words, r) => (
        <div key={r} style={{ display: "flex", gap: size * 0.24, lineHeight: 1.12 }}>
          {words.map((wd, i) => {
            const s = at + n++ * step;
            const appear = ease(frame, s, 4);
            const settle = ease(frame, s + 3, 9);
            return (
              <span
                key={i}
                style={{
                  fontFamily: sans,
                  fontSize: size,
                  fontWeight: weight,
                  letterSpacing: "-0.03em",
                  opacity: appear,
                  color: mix(grey, color, settle),
                  translate: `0px ${(1 - settle) * size * 0.1}px`,
                }}
              >
                {wd}
              </span>
            );
          })}
        </div>
      ))}
    </div>
  );
};

const mix = (a: string, c: string, t: number) => {
  const pa = [1, 3, 5].map((i) => parseInt(a.slice(i, i + 2), 16));
  const pc = [1, 3, 5].map((i) => parseInt(c.slice(i, i + 2), 16));
  return `rgb(${pa.map((v, i) => Math.round(v + (pc[i] - v) * t)).join(",")})`;
};

// "Introducing": letters fly in from scattered positions and settle.
const Scatter: React.FC<{ at: number; out: number; text: string; size?: number }> = ({ at, out, text, size = 150 }) => {
  const frame = useCurrentFrame();
  const seed = (i: number, k: number) => {
    const x = Math.sin(i * 12.9898 + k * 78.233) * 43758.5453;
    return x - Math.floor(x) - 0.5;
  };
  return (
    <div style={{ display: "flex" }}>
      {text.split("").map((ch, i) => {
        const pin = ease(frame, at + i * 1.2, 12);
        const pout = interpolate(frame, [out + i * 0.8, out + i * 0.8 + 10], [0, 1], { ...clamp, easing: curve.exit });
        const dx = seed(i, 1) * 260 * (1 - pin) + seed(i, 3) * 340 * pout;
        const dy = seed(i, 2) * 220 * (1 - pin) + (seed(i, 4) + 0.6) * 260 * pout;
        return (
          <span
            key={i}
            style={{
              fontFamily: sans,
              fontSize: size,
              fontWeight: 500,
              letterSpacing: "-0.03em",
              display: "inline-block",
              color: mix(grey, ink, pin),
              opacity: Math.min(pin * 1.4, 1) * (1 - pout),
              translate: `${dx}px ${dy}px`,
              whiteSpace: "pre",
            }}
          >
            {ch}
          </span>
        );
      })}
    </div>
  );
};

// ---------- pieces ----------

const Mark: React.FC<{ size: number }> = ({ size }) => (
  <div
    style={{
      width: size,
      height: size,
      borderRadius: size * 0.27,
      background: blue,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      boxShadow: "inset 0 -6px 14px rgba(0,0,0,0.12), 0 10px 30px rgba(10,132,255,0.25)",
    }}
  >
    <svg width={size * 0.66} height={size * 0.66} viewBox="0 0 100 100">
      <circle cx="50" cy="50" r="26" fill="#fff" />
      <ellipse cx="50" cy="50" rx="45" ry="14" fill="none" stroke="#fff" strokeWidth="7" transform="rotate(-24 50 50)" />
    </svg>
  </div>
);

const Card: React.FC<{ style?: React.CSSProperties; children: React.ReactNode }> = ({ style, children }) => (
  <div style={{ background: "#fff", border: `1px solid ${line}`, borderRadius: 22, boxShadow: cardShadow, ...style }}>{children}</div>
);

// Card with a 3D pose that eases between two poses.
const Tilt: React.FC<{
  from: [number, number, number, number, number, number]; // rx, ry, rz, x, y, scale
  to: [number, number, number, number, number, number];
  at: number;
  dur: number;
  opacity?: number;
  children: React.ReactNode;
}> = ({ from, to, at, dur, opacity = 1, children }) => {
  const frame = useCurrentFrame();
  const t = interpolate(frame, [at, at + dur], [0, 1], { ...clamp, easing: curve.glide });
  const v = from.map((a, i) => a + (to[i] - a) * t);
  return (
    <div
      style={{
        position: "absolute",
        left: "50%",
        top: "50%",
        opacity,
        transform: `translate(-50%, -50%) translate(${v[3]}px, ${v[4]}px) perspective(1600px) rotateX(${v[0]}deg) rotateY(${v[1]}deg) rotateZ(${v[2]}deg) scale(${v[5]})`,
      }}
    >
      {children}
    </div>
  );
};

const Center: React.FC<{ y?: number; x?: number; opacity?: number; children: React.ReactNode }> = ({ y = 0, x = 0, opacity = 1, children }) => (
  <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", translate: `${x}px ${y}px`, opacity }}>{children}</AbsoluteFill>
);

const Pin: React.FC = () => (
  <div style={{ width: 64, height: 64, borderRadius: 999, border: `1px solid ${line}`, display: "flex", alignItems: "center", justifyContent: "center", background: "#fafafa" }}>
    <svg width="30" height="30" viewBox="0 0 24 24">
      <path d="M12 2c-3.9 0-7 3.1-7 7 0 5.2 7 13 7 13s7-7.8 7-13c0-3.9-3.1-7-7-7z" fill={ink} />
      <circle cx="12" cy="9" r="2.6" fill="#fff" />
    </svg>
  </div>
);

const Chip: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div style={{ fontFamily: sans, fontSize: 26, color: "#6e6e73", border: `1px solid ${line}`, borderRadius: 10, padding: "8px 16px", background: "#fafafa", whiteSpace: "nowrap" }}>{children}</div>
);

const PlaceCard: React.FC<{ title: string; sub: string; chip: string; width?: number }> = ({ title, sub, chip, width = 860 }) => (
  <Card style={{ width, padding: "26px 30px", display: "flex", alignItems: "center", gap: 24 }}>
    <Pin />
    <div style={{ flex: 1 }}>
      <div style={{ fontFamily: sans, fontSize: 36, fontWeight: 600, color: ink, letterSpacing: "-0.01em" }}>{title}</div>
      <div style={{ fontFamily: sans, fontSize: 28, color: grey, marginTop: 4 }}>↳ {sub}</div>
    </div>
    <Chip>{chip}</Chip>
  </Card>
);

const Stat: React.FC<{ label: string; value: string; dot: string }> = ({ label, value, dot }) => (
  <Card style={{ width: 330, padding: "22px 26px" }}>
    <div style={{ fontFamily: sans, fontSize: 24, color: "#6e6e73" }}>{label}</div>
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 10 }}>
      <span style={{ width: 14, height: 14, borderRadius: 4, background: dot }} />
      <span style={{ fontFamily: sans, fontSize: 40, fontWeight: 600, color: ink, letterSpacing: "-0.02em" }}>{value}</span>
    </div>
  </Card>
);

const Tile: React.FC<{ name: string }> = ({ name }) => (
  <Card style={{ width: 170, height: 170, borderRadius: 40, display: "flex", alignItems: "center", justifyContent: "center" }}>
    <span style={{ fontFamily: sans, fontSize: 30, fontWeight: 600, color: ink, letterSpacing: "-0.01em" }}>{name}</span>
  </Card>
);

// Typed text with a caret.
// cpf: characters per frame
const typed = (frame: number, at: number, text: string, cpf = 0.5) => text.slice(0, Math.max(0, Math.min(text.length, Math.floor((frame - at) * cpf))));
const Caret: React.FC = () => {
  const frame = useCurrentFrame();
  return <span style={{ display: "inline-block", width: 3, height: "1em", background: ink, marginLeft: 3, verticalAlign: "-0.12em", opacity: Math.floor(frame / 15) % 2 === 0 ? 1 : 0 }} />;
};

// ---------- timeline (beats) ----------

const T = {
  intro: 0,
  brand: b(5),
  turn: b(9),
  pill: b(15),
  real: b(19),
  cmd: b(31),
  world: b(39),
  engines: b(51),
  build: b(57),
  end: b(61),
  total: b(71),
};

const TOUR = [
  { file: "models/tokyo-tower.glb", title: "Tokyo Tower", sub: "Minato, Tokyo", dist: 820, targetY: 105 },
  { file: "models/new-york-empire-state.glb", title: "Empire State Building", sub: "Manhattan, New York", dist: 980, targetY: 140 },
  { file: "models/paris-eiffel.glb", title: "Eiffel Tower", sub: "Paris, France", dist: 820, targetY: 105 },
  { file: "models/sydney-opera-house.glb", title: "Sydney Opera House", sub: "Sydney, Australia", dist: 520, targetY: 18 },
];
const TOUR_LEN = b(3);

export const LIGHT_TOTAL = T.total;

export const Light: React.FC = () => {
  const frame = useCurrentFrame();

  // Scene 3: "Turn any place" moves up when the card arrives, both leave together.
  const turnUp = ease(frame, T.turn + b(2), b(1));
  const turnOut = ease(frame, T.pill - b(1), b(1));

  // Scene 6: command field.
  const cmdText = '"Tokyo Tower"';
  const typedCmd = typed(frame, T.cmd + b(3), cmdText, 0.5);

  return (
    <AbsoluteFill>
      <Grid />

      {/* 1: Introducing */}
      <Sequence from={T.intro} durationInFrames={T.brand}>
        <Center>
          <Scatter at={2} out={b(4.3)} text="Introducing" />
        </Center>
      </Sequence>

      {/* 2: mark + name */}
      <Sequence from={T.brand} durationInFrames={T.turn - T.brand}>
        <BrandLockup />
      </Sequence>

      {/* 3: Turn any place + place card */}
      <Sequence from={T.turn} durationInFrames={T.pill - T.turn}>
        <Center y={-turnUp * 120 - turnOut * 80} opacity={1 - turnOut}>
          <Reveal at={0} step={b(0.5)} text="Turn any place" />
        </Center>
        <Tilt from={[30, 0, 0, 0, 380, 0.96]} to={[0, 0, 0, 0, 60, 1]} at={b(2)} dur={b(1)} opacity={ease(frame - T.turn, b(2), 6) * (1 - turnOut)}>
          <PlaceCard title="Brandenburger Tor" sub="Pariser Platz, Berlin" chip="3D · 90 tiles" />
        </Tilt>
      </Sequence>

      {/* 4: into a game-ready 3D model */}
      <Sequence from={T.pill} durationInFrames={T.real - T.pill}>
        <PillScene />
      </Sequence>

      {/* 5: the real place builds, then grows its trees */}
      <Sequence from={T.real} durationInFrames={T.cmd - T.real}>
        <RealScene />
      </Sequence>

      {/* 6: one command */}
      <Sequence from={T.cmd} durationInFrames={T.world - T.cmd}>
        <CommandScene typedCmd={typedCmd} full={cmdText} />
      </Sequence>

      {/* 7: world tour */}
      {TOUR.map((c, i) => (
        <Sequence key={c.title} from={T.world + i * TOUR_LEN} durationInFrames={TOUR_LEN}>
          <City3D
            file={c.file}
            build={{ tilesStart: 0, tilesEnd: 12, splatStart: 6, splatEnd: 18 }}
            orbit={{ startDeg: 50 + i * 25, sweepDeg: 22, elevDeg: 20, dist: c.dist * 1.05, distEnd: c.dist * 0.95, targetY: c.targetY, duration: TOUR_LEN }}
          />
          <TourLabel title={c.title} sub={c.sub} />
        </Sequence>
      ))}
      <Sequence from={T.world} durationInFrames={T.engines - T.world}>
        <AbsoluteFill style={{ alignItems: "center", paddingTop: 70 }}>
          <Reveal at={0} step={b(0.5)} text="1,400+ regions in 3D" size={100} />
        </AbsoluteFill>
      </Sequence>

      {/* 8: drops into every engine */}
      <Sequence from={T.engines} durationInFrames={T.build - T.engines}>
        <EngineScene />
      </Sequence>

      {/* 9: Build your world with */}
      <Sequence from={T.build} durationInFrames={T.end - T.build}>
        <BuildScene />
      </Sequence>

      {/* 10: end card */}
      <Sequence from={T.end}>
        <EndScene />
      </Sequence>

      {/* sound */}
      <Audio src={staticFile("audio/music-bed.wav")} volume={(f) => interpolate(f, [0, 6, T.total - b(3), T.total], [0, 0.9, 0.9, 0], clamp)} />
      {Array.from({ length: cmdText.length }, (_, i) => (
        <Sequence key={i} from={T.cmd + b(3) + i * 2} durationInFrames={10}>
          <Audio src={staticFile(`audio/key${(i % 3) + 1}.mp3`)} volume={0.28} playbackRate={0.95 + (i % 4) * 0.03} />
        </Sequence>
      ))}
      {Array.from({ length: 26 }, (_, i) => (
        <Sequence key={`u${i}`} from={T.end + b(4) + i * 2} durationInFrames={10}>
          <Audio src={staticFile(`audio/key${(i % 3) + 1}.mp3`)} volume={0.22} playbackRate={0.95 + (i % 4) * 0.03} />
        </Sequence>
      ))}
      {[T.turn + b(2), T.real + b(2), T.real + b(4), T.real + b(6), T.cmd + b(1.5)].map((f, i) => (
        <Sequence key={`s${i}`} from={f - 2} durationInFrames={16}>
          <Audio src={staticFile(`audio/snap${(i % 3) + 1}.mp3`)} volume={0.16} />
        </Sequence>
      ))}
    </AbsoluteFill>
  );
};

const BrandLockup: React.FC = () => {
  const frame = useCurrentFrame();
  const pop = ease(frame, 0, 10);
  const slide = ease(frame, b(1), b(1));
  const out = ease(frame, b(3.5), 8);
  return (
    <Center y={-out * 60} opacity={1 - out}>
      <div style={{ display: "flex", alignItems: "center", gap: 34 * slide }}>
        <div style={{ scale: `${0.6 + 0.4 * pop}`, opacity: pop }}>
          <Mark size={170} />
        </div>
        <div style={{ width: 500 * slide, overflow: "hidden", whiteSpace: "nowrap" }}>
          <Reveal at={b(1.2)} step={b(0.5)} text="Orbis" size={160} weight={600} align="left" />
        </div>
      </div>
    </Center>
  );
};

const PillScene: React.FC = () => {
  const frame = useCurrentFrame();
  const inP = ease(frame, 0, 10);
  const sweep = interpolate(frame, [4, b(2.5)], [-30, 130], { ...clamp, easing: curve.glide });
  const out = ease(frame, b(3.5), 8);
  return (
    <Center opacity={1 - out} y={out * -40}>
      <div
        style={{
          scale: `${0.92 + 0.08 * inP}`,
          opacity: inP,
          padding: "26px 54px",
          borderRadius: 26,
          border: `1px solid ${line}`,
          background: `linear-gradient(90deg, #ffffff 0%, #ffffff ${sweep - 25}%, #e8f1ff ${sweep}%, #ffffff ${sweep + 25}%, #ffffff 100%)`,
          boxShadow: cardShadow,
        }}
      >
        <Reveal at={2} step={b(0.5)} text="into a game-ready 3D model" size={92} />
      </div>
    </Center>
  );
};

const RealScene: React.FC = () => {
  const frame = useCurrentFrame();
  const len = T.cmd - T.real;
  const first = frame < b(5.5);
  const out = ease(frame, len - 8, 8);
  return (
    <AbsoluteFill style={{ opacity: 1 - out }}>
      <AbsoluteFill style={{ translate: "0px 70px" }}>
        <City3D
          file="models/berlin.glb"
          build={{ tilesStart: 0, tilesEnd: b(3), splatStart: b(5.5), splatEnd: b(8) }}
          orbit={{ startDeg: -55, sweepDeg: 60, elevDeg: 40, elevEndDeg: 22, dist: 330, distEnd: 175, targetY: 8, duration: len }}
        />
      </AbsoluteFill>
      <AbsoluteFill style={{ alignItems: "center", paddingTop: 70 }}>
        {first ? <Reveal key="a" at={0} step={b(0.5)} text="A real place, in 3D" size={104} /> : <Reveal key="b" at={b(5.5)} step={b(0.5)} text="right down to the trees" size={104} />}
      </AbsoluteFill>
      <Tilt from={[0, 30, -6, -980, 140, 0.9]} to={[8, 18, -5, -560, 170, 1]} at={b(2)} dur={b(1.5)} opacity={ease(frame, b(2), 6)}>
        <Stat label="Tiles" value="90" dot="#0a84ff" />
      </Tilt>
      <Tilt from={[0, -30, 6, 980, 60, 0.9]} to={[8, -18, 5, 640, 90, 1]} at={b(4)} dur={b(1.5)} opacity={ease(frame, b(4), 6)}>
        <Stat label="Area" value="200 × 200 m" dot="#bf5af2" />
      </Tilt>
      <Tilt from={[0, -30, 6, 980, 300, 0.9]} to={[8, -18, 4, 600, 300, 1]} at={b(6)} dur={b(1.5)} opacity={ease(frame, b(6), 6)}>
        <Stat label="Tree splats" value="252,091" dot="#30d158" />
      </Tilt>
    </AbsoluteFill>
  );
};

const FORMATS = [
  { k: "GLB", d: "Unity · Unreal · Godot · Blender" },
  { k: "USDZ", d: "Xcode · RealityKit · visionOS" },
  { k: "PLY", d: "Gaussian splats" },
];

const CommandScene: React.FC<{ typedCmd: string; full: string }> = ({ typedCmd, full }) => {
  const frame = useCurrentFrame();
  const len = T.world - T.cmd;
  const card = ease(frame, b(1.5), b(1));
  const textX = interpolate(card, [0, 1], [0, -470]);
  const done = typedCmd.length === full.length;
  const enter = frame >= b(6.5);
  const out = ease(frame, len - 8, 8);
  return (
    <AbsoluteFill style={{ opacity: 1 - out }}>
      <Center x={textX}>
        <Reveal at={0} step={b(0.5)} lines={["Export with", "one command"]} text="" size={110} align="left" />
      </Center>
      <Tilt from={[20, -30, 0, 760, 40, 0.92]} to={[6, -14, 0, 330, 20, 1]} at={b(1.5)} dur={b(1.2)} opacity={card}>
        <Card style={{ width: 860, overflow: "hidden" }}>
          <div style={{ padding: "28px 32px", fontFamily: mono, fontSize: 32, color: ink, borderBottom: `1px solid ${line}`, whiteSpace: "nowrap" }}>
            <span style={{ color: grey }}>$ </span>orbis export --place {typedCmd}
            {!enter ? <Caret /> : null}
          </div>
          {!done
            ? FORMATS.map((f, i) => (
                <div key={f.k} style={{ display: "flex", gap: 22, alignItems: "baseline", padding: "20px 32px", background: i === 0 ? "#f5f5f7" : "#fff", opacity: ease(frame, b(2) + i * 4, 8) }}>
                  <span style={{ fontFamily: mono, fontSize: 28, fontWeight: 600, color: ink, width: 100 }}>{f.k}</span>
                  <span style={{ fontFamily: sans, fontSize: 28, color: "#6e6e73" }}>{f.d}</span>
                </div>
              ))
            : (
              <div style={{ display: "flex", gap: 18, alignItems: "center", padding: "22px 32px", background: "#f5f5f7" }}>
                <span style={{ fontFamily: sans, fontSize: 28, color: ink }}>↳ Tokyo Tower, Minato, Tokyo</span>
                <span style={{ marginLeft: "auto", fontFamily: sans, fontSize: 24, color: enter ? "#fff" : "#6e6e73", background: enter ? blue : "#fff", border: `1px solid ${enter ? blue : line}`, borderRadius: 10, padding: "6px 14px" }}>
                  {enter ? "Exporting…" : "↵ Enter"}
                </span>
              </div>
            )}
        </Card>
      </Tilt>
    </AbsoluteFill>
  );
};

const TourLabel: React.FC<{ title: string; sub: string }> = ({ title, sub }) => {
  const frame = useCurrentFrame();
  const p = ease(frame, 2, 10);
  return (
    <div style={{ position: "absolute", left: 110, bottom: 100, opacity: p, translate: `0px ${(1 - p) * 30}px` }}>
      <PlaceCard title={title} sub={sub} chip="GLB · USDZ · PLY" width={720} />
    </div>
  );
};

const ENGINES = [
  { n: "Unity", x: -620, y: -250 },
  { n: "Unreal", x: 640, y: -230 },
  { n: "Godot", x: -720, y: 160 },
  { n: "Blender", x: 700, y: 190 },
  { n: "Xcode", x: -280, y: 330 },
  { n: "three.js", x: 330, y: 340 },
];

const EngineScene: React.FC = () => {
  const frame = useCurrentFrame();
  const out = ease(frame, b(5.5), 8);
  return (
    <AbsoluteFill style={{ opacity: 1 - out }}>
      <Center y={-30}>
        <Reveal at={0} step={b(0.5)} lines={["Drops right into", "your engine"]} text="" size={120} />
        <div style={{ marginTop: 26, opacity: ease(frame, b(2.5), 10) }}>
          <span style={{ fontFamily: sans, fontSize: 40, color: "#6e6e73" }}>GLB · USDZ · Gaussian splat PLY</span>
        </div>
      </Center>
      {ENGINES.map((e, i) => {
        const p = ease(frame, b(1) + i * 4, 12);
        const float = Math.sin((frame + i * 20) / 22) * 8;
        return (
          <div
            key={e.n}
            style={{
              position: "absolute",
              left: "50%",
              top: "50%",
              opacity: p,
              transform: `translate(-50%, -50%) translate(${e.x * (0.85 + 0.15 * p)}px, ${e.y * (0.85 + 0.15 * p) + float}px) perspective(1200px) rotateY(${(e.x > 0 ? -1 : 1) * 14}deg) rotateZ(${(i % 2 ? 1 : -1) * 4}deg) scale(${0.8 + 0.2 * p})`,
            }}
          >
            <Tile name={e.n} />
          </div>
        );
      })}
    </AbsoluteFill>
  );
};

const BuildScene: React.FC = () => {
  const frame = useCurrentFrame();
  // The one coloured word comes in with motion blur, like the reference's "Supercharge".
  const p = ease(frame, 0, 10);
  const out = ease(frame, b(3.4), 6);
  return (
    <Center opacity={1 - out}>
      <div style={{ display: "flex", gap: 26, alignItems: "baseline" }}>
        <span
          style={{
            fontFamily: sans,
            fontSize: 132,
            fontWeight: 600,
            letterSpacing: "-0.03em",
            color: blue,
            filter: p < 1 ? `blur(${(1 - p) * 10}px)` : undefined,
            translate: `${(1 - p) * -120}px 0px`,
            opacity: p,
          }}
        >
          Build
        </span>
        <Reveal at={b(1)} step={b(0.5)} text="real worlds with" />
      </div>
    </Center>
  );
};

const COLLAGE = [
  { src: "screens/card-tokyo.png", x: -740, y: -330, r: -7 },
  { src: "screens/card-eiffel.png", x: 760, y: -320, r: 6 },
  { src: "screens/card-sydney.png", x: -780, y: 340, r: 5 },
  { src: "screens/card-berlin-trees.png", x: 770, y: 330, r: -6 },
  { src: "screens/card-empire.png", x: 0, y: -470, r: 2 },
];

const EndScene: React.FC = () => {
  const frame = useCurrentFrame();
  const url = "github.com/Henkek221/orbis";
  const u = typed(frame, b(4), url, 0.5);
  return (
    <AbsoluteFill>
      {COLLAGE.map((c, i) => {
        const p = ease(frame, i * 3, b(1.5));
        return (
          <div
            key={c.src}
            style={{
              position: "absolute",
              left: "50%",
              top: "50%",
              opacity: p,
              transform: `translate(-50%, -50%) translate(${c.x * (1.12 - 0.12 * p)}px, ${c.y * (1.12 - 0.12 * p)}px) perspective(1600px) rotateX(22deg) rotateZ(${c.r}deg)`,
            }}
          >
            <div style={{ borderRadius: 18, overflow: "hidden", border: `1px solid ${line}`, boxShadow: cardShadow, background: "#fff" }}>
              <Img src={staticFile(c.src)} style={{ width: 560, display: "block" }} />
            </div>
          </div>
        );
      })}
      <AbsoluteFill style={{ background: "radial-gradient(ellipse 36% 32% at 50% 50%, rgba(255,255,255,0.97) 45%, rgba(255,255,255,0) 100%)" }} />
      <Center>
        <div style={{ display: "flex", alignItems: "center", gap: 30, opacity: ease(frame, 0, 8), scale: `${0.94 + 0.06 * ease(frame, 0, 12)}` }}>
          <Mark size={120} />
          <span style={{ fontFamily: sans, fontSize: 150, fontWeight: 650, letterSpacing: "-0.045em", color: ink }}>Orbis</span>
        </div>
        <div style={{ marginTop: 20 }}>
          <Reveal at={b(1.5)} step={b(0.5)} text="It all starts with a place" size={64} color="#3a3a3c" />
        </div>
        <div style={{ marginTop: 18, fontFamily: sans, fontSize: 50, color: ink, minHeight: 64, opacity: frame >= b(3.6) ? 1 : 0 }}>
          ↳ {u}
          <Caret />
        </div>
      </Center>
    </AbsoluteFill>
  );
};
