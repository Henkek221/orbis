import React from "react";
import { AbsoluteFill, Audio, Composition, Img, Sequence, interpolate, staticFile, useCurrentFrame } from "remotion";
import { City3D } from "./City3D";
import { EndCard, Glow, Words, w } from "./Kinetic";
import { COMMAND, ENTER, Terminal, typedAt } from "./Terminal";
import { clamp, color, curve, font, FPS, H, W } from "./theme";

// Music: Mixkit tracks, measured (tempo, drop) with tools/beat.py. The drop
// lands on Enter; every later section is a whole number of beats.
const TRACKS = {
  a: { file: "audio/music-a.mp3", bpm: 127.95, drop: 30.128 },
  b: { file: "audio/music-b.mp3", bpm: 120.01, drop: 32.099 },
  c: { file: "audio/music-c.mp3", bpm: 125.9, drop: 22.866 },
  d: { file: "audio/music-d.mp3", bpm: 120.21, drop: 31.983 },
} as const;
export type TrackId = keyof typeof TRACKS;
const TRACK: TrackId = (process.env.REMOTION_TRACK as TrackId) || "a";
const MUSIC = TRACKS[TRACK] ?? TRACKS.a;
const BEAT = (60 / MUSIC.bpm) * FPS;
const beats = (n: number) => Math.round(n * BEAT);

// Timeline: opener, typing, drop on Enter, build, world tour, formats, trees, end.
const OPEN = beats(8);
const DROP = OPEN + ENTER;
const CITY_IN = DROP;
const BUILD = beats(12);
const WORLD_IN = CITY_IN + BUILD;
const CITY_LEN = beats(3);
// Framing: aim at about a third of the landmark's height and stand back far
// enough that the whole model, including its base, stays in frame.
const CITIES = [
  { file: "models/new-york-empire-state.glb", place: "Empire State Building", dist: 980, targetY: 140 },
  { file: "models/paris-eiffel.glb", place: "Eiffel Tower", dist: 820, targetY: 105 },
  { file: "models/tokyo-tower.glb", place: "Tokyo Tower", dist: 820, targetY: 105 },
  { file: "models/sydney-opera-house.glb", place: "Sydney Opera House", dist: 520, targetY: 18 },
  { file: "models/berlin-fernsehturm.glb", place: "Fernsehturm Berlin", dist: 900, targetY: 115 },
];
const FORMATS_IN = WORLD_IN + CITIES.length * CITY_LEN;
const SCREEN_LEN = beats(3);
const FORMATS_LEN = 3 * SCREEN_LEN;
const TREES_IN = FORMATS_IN + FORMATS_LEN;
const TREES_LEN = beats(8);
const BLOOM = TREES_IN + beats(1);
const END_IN = TREES_IN + TREES_LEN;
const TOTAL = END_IN + beats(10);
const MUSIC_TRIM = Math.round(MUSIC.drop * FPS - DROP);

export const MyComposition = () => (
  <Composition id="OrbisLaunch" component={Launch} durationInFrames={TOTAL} fps={FPS} width={W} height={H} />
);

// World tour: the command changes per city and the city builds from it.
const PlaceCommand: React.FC<{ place: string }> = ({ place }) => {
  const frame = useCurrentFrame();
  const cmd = `orbis export --place "${place}"`;
  const typed = Math.min(cmd.length, Math.floor(frame * 4.5));
  return (
    <div style={{ position: "absolute", left: 120, bottom: 96, fontFamily: font.mono, fontSize: 50, color: color.text, background: "rgba(11,15,26,0.9)", padding: "18px 32px", borderRadius: 18 }}>
      <span style={{ color: color.green }}>~ </span>
      {cmd.slice(0, typed)}
    </div>
  );
};

// One export, opened in three real renderers, one after another, large.
const Screen: React.FC<{ src: string; file: string; apps: string }> = ({ src, file, apps }) => {
  const frame = useCurrentFrame();
  return (
    <AbsoluteFill style={{ alignItems: "center", paddingTop: 270 }}>
      <div
        style={{
          width: 1180,
          height: 700,
          borderRadius: 22,
          overflow: "hidden",
          background: "#0b0f1a",
          boxShadow: "0 40px 120px rgba(0,0,0,0.7), inset 0 0 0 1px rgba(255,255,255,0.08)",
          scale: interpolate(frame, [0, 6], [0.94, 1], { ...clamp, easing: curve.land }),
        }}
      >
        <Img src={staticFile(src)} style={{ width: 1180, height: 700, objectFit: "cover", objectPosition: "center top" }} />
      </div>
      <div style={{ marginTop: 26, display: "flex", gap: 30, alignItems: "baseline" }}>
        <span style={{ fontFamily: font.mono, fontSize: 48, color: color.blue }}>{file}</span>
        <span style={{ fontFamily: font.display, fontSize: 44, fontWeight: 500, color: color.text }}>{apps}</span>
      </div>
    </AbsoluteFill>
  );
};

// The terminal leaves once the export is done, so the Gate is free.
const TerminalExit: React.FC<{ at: number; children: React.ReactNode }> = ({ at, children }) => {
  const frame = useCurrentFrame();
  return (
    <AbsoluteFill
      style={{
        translate: interpolate(frame, [at, at + 12], ["0px 0px", "0px 260px"], { ...clamp, easing: curve.exit }),
        opacity: interpolate(frame, [at, at + 12], [1, 0], clamp),
      }}
    >
      {children}
    </AbsoluteFill>
  );
};

const NO_SPLATS = { splatStart: 99999, splatEnd: 100000 };

// Sound: real CC0 recordings, three keys rotated with small pitch changes.
const Keys: React.FC = () => (
  <>
    {Array.from({ length: COMMAND.length }, (_, i) => (
      <Sequence key={i} from={Math.max(0, typedAt(i + 1) - 3)} durationInFrames={12}>
        <Audio src={staticFile(`audio/key${(i % 3) + 1}.mp3`)} volume={0.35 + ((i * 7) % 5) * 0.04} playbackRate={0.94 + ((i * 5) % 7) * 0.02} />
      </Sequence>
    ))}
  </>
);

export const Launch: React.FC = () => {
  const cuts = CITIES.map((_, i) => WORLD_IN + i * CITY_LEN);
  return (
    <AbsoluteFill style={{ background: color.bg }}>
      {/* 0: opener, type only, on a soft light */}
      <Glow from={0} until={OPEN + 4} />
      <Words at={beats(1)} until={OPEN} step={beats(0.5)} size={128} lines={[w("Turn the real world"), w("into game worlds.", true)]} />

      {/* 1–2: the export builds Berlin, framed up and right of the terminal; it ends on the Gate */}
      <Sequence from={CITY_IN} durationInFrames={BUILD}>
        <AbsoluteFill style={{ translate: "230px -30px" }}>
          <City3D
            file="models/berlin.glb"
            only="mesh"
            build={{ tilesStart: 0, tilesEnd: 110, ...NO_SPLATS }}
            orbit={{ startDeg: -55, sweepDeg: 55, elevDeg: 42, elevEndDeg: 20, dist: 330, distEnd: 150, targetY: 8, duration: BUILD }}
          />
        </AbsoluteFill>
      </Sequence>
      <Sequence from={OPEN} durationInFrames={ENTER + 10 + BUILD}>
        <TerminalExit at={ENTER + beats(8)}>
          <Terminal progressStart={ENTER} progressEnd={ENTER + beats(7)} shrinkAt={ENTER} />
        </TerminalExit>
      </Sequence>
      <Words at={CITY_IN + 2} until={CITY_IN + beats(4)} step={beats(0.5)} top={96} lines={[w("One command.")]} />
      <Words at={CITY_IN + beats(4)} until={WORLD_IN} step={beats(0.5)} top={96} lines={[w("A real place.", true), w("From Apple’s 3D map data.").map((x) => ({ ...x, light: true }))]} />

      {/* 3: world tour; each city is one command that builds it */}
      {CITIES.map((c, i) => (
        <Sequence key={c.place} from={cuts[i]} durationInFrames={CITY_LEN}>
          <City3D
            file={c.file}
            build={{ tilesStart: 4, tilesEnd: 16, splatStart: 10, splatEnd: 20 }}
            orbit={{ startDeg: 60 + i * 20, sweepDeg: 20, elevDeg: 22, dist: c.dist, distEnd: c.dist * 0.93, targetY: c.targetY, duration: CITY_LEN }}
          />
          <AbsoluteFill style={{ background: "linear-gradient(0deg, rgba(5,6,8,0.9) 0%, rgba(5,6,8,0) 22%)" }} />
          <PlaceCommand place={c.place} />
        </Sequence>
      ))}
      <Words at={WORLD_IN} until={FORMATS_IN} step={beats(0.5)} size={96} top={90} lines={[[{ t: "1,400+", accent: true }, ...w("regions in 3D.")]]} />

      {/* 4: one export, opened in three real renderers */}
      <Sequence from={FORMATS_IN} durationInFrames={SCREEN_LEN}>
        <Screen src="screens/glb-threejs.png" file="berlin.glb" apps="three.js · Unity · Unreal · Godot · Blender" />
      </Sequence>
      <Sequence from={FORMATS_IN + SCREEN_LEN} durationInFrames={SCREEN_LEN}>
        <Screen src="screens/usdz-xcode.png" file="berlin.usdz" apps="Xcode · RealityKit · visionOS" />
      </Sequence>
      <Sequence from={FORMATS_IN + 2 * SCREEN_LEN} durationInFrames={FORMATS_LEN - 2 * SCREEN_LEN}>
        <Screen src="screens/ply-splats.png" file="berlin-splats.ply" apps="Gaussian splat viewers" />
      </Sequence>
      <Words at={FORMATS_IN} until={TREES_IN} step={beats(0.5)} size={100} top={96} lines={[[...w("One export."), ...w("Every engine.", true)]]} />

      {/* 5: the flat map grows Apple's new splat trees */}
      <Sequence from={TREES_IN} durationInFrames={TREES_LEN}>
        <City3D
          file="models/berlin.glb"
          build={{ tilesStart: -100, tilesEnd: -100, splatStart: BLOOM - TREES_IN, splatEnd: BLOOM - TREES_IN + 45 }}
          orbit={{ startDeg: 20, sweepDeg: 30, elevDeg: 30, elevEndDeg: 26, dist: 115, distEnd: 95, targetX: -70, targetZ: -60, targetY: -2, duration: TREES_LEN }}
        />
        <AbsoluteFill style={{ background: "linear-gradient(180deg, rgba(5,6,8,0.85) 0%, rgba(5,6,8,0.4) 28%, rgba(5,6,8,0) 45%)" }} />
      </Sequence>
      <Words at={BLOOM} until={END_IN} step={beats(0.5)} top={96} lines={[[...w("Even the"), ...w("trees.", true)], w("Apple’s new splats, soft in every engine.").map((x) => ({ ...x, light: true }))]} />

      {/* 6: Berlin from above, now with trees; the prompt waits for your place */}
      <Sequence from={END_IN} durationInFrames={TOTAL - END_IN}>
        <City3D file="models/berlin.glb" orbit={{ startDeg: 230, sweepDeg: 25, elevDeg: 62, elevEndDeg: 72, dist: 330, distEnd: 430, targetY: 5, duration: TOTAL - END_IN }} />
        <AbsoluteFill style={{ background: "radial-gradient(ellipse at center, rgba(5,6,8,0.62) 0%, rgba(5,6,8,0.86) 70%)" }} />
      </Sequence>
      <EndCard at={END_IN + beats(1)} step={beats(1)} />

      {/* Sound */}
      <Audio
        src={staticFile(MUSIC.file)}
        trimBefore={Math.max(0, MUSIC_TRIM)}
        volume={(f) => interpolate(f, [0, 20, TOTAL - beats(4), TOTAL], [0, 0.8, 0.8, 0], clamp)}
      />
      <Sequence from={OPEN}>
        <Keys />
      </Sequence>
      <Sequence from={DROP - 1} durationInFrames={20}>
        <Audio src={staticFile("audio/enter1.mp3")} volume={0.7} />
      </Sequence>
      {Array.from({ length: 9 }, (_, i) => (
        <Sequence key={i} from={CITY_IN + i * beats(0.5)} durationInFrames={18}>
          <Audio src={staticFile(`audio/snap${(i % 3) + 1}.mp3`)} volume={0.18 + (i % 3) * 0.05} playbackRate={0.95 + (i % 4) * 0.03} />
        </Sequence>
      ))}
      <Sequence from={DROP - 20} durationInFrames={120}>
        <Audio src={staticFile("audio/impact1.mp3")} trimBefore={30} volume={0.3} />
      </Sequence>
      {cuts.map((c, i) => (
        <Sequence key={c} from={c - 6} durationInFrames={30}>
          <Audio src={staticFile(`audio/whoosh${(i % 2) + 1}.mp3`)} volume={0.3} playbackRate={0.96 + (i % 3) * 0.04} />
        </Sequence>
      ))}
      {[0, 1, 2].map((b, i) => (
        <Sequence key={b} from={FORMATS_IN + b * SCREEN_LEN} durationInFrames={18}>
          <Audio src={staticFile(`audio/snap${i + 1}.mp3`)} volume={0.3} />
        </Sequence>
      ))}
    </AbsoluteFill>
  );
};
