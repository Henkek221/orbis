// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

import React, { useEffect, useMemo, useState } from "react";
import { ThreeCanvas } from "@remotion/three";
import { useThree } from "@react-three/fiber";
import { continueRender, delayRender, interpolate, staticFile, useCurrentFrame } from "remotion";
import * as THREE from "three";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { clamp, curve, H, W } from "./theme";

// One loaded GLB per file for the whole render; scenes clone it.
const cache = new Map<string, Promise<THREE.Group>>();
const load = (file: string) => {
  if (!cache.has(file)) {
    cache.set(
      file,
      new GLTFLoader().loadAsync(staticFile(file)).then((g) => g.scene),
    );
  }
  return cache.get(file)!;
};

export type Build = { tilesStart: number; tilesEnd: number; splatStart: number; splatEnd: number };
export type Orbit = {
  startDeg: number;
  sweepDeg: number;
  elevDeg: number;
  elevEndDeg?: number;
  dist: number;
  distEnd?: number;
  targetY?: number;
  // Shift the look-at point, in metres east (x) and south (z).
  targetX?: number;
  targetZ?: number;
  duration: number;
};

// Camera pose on the orbit at a frame. Constant angular speed: the camera
// glides, it never eases in mid-shot.
export const orbitPose = (orbit: Orbit, frame: number) => {
  const t = interpolate(frame, [0, orbit.duration], [0, 1], clamp);
  const az = ((orbit.startDeg + orbit.sweepDeg * t) * Math.PI) / 180;
  const el = ((orbit.elevDeg + ((orbit.elevEndDeg ?? orbit.elevDeg) - orbit.elevDeg) * curve.glide(t)) * Math.PI) / 180;
  const d = orbit.dist + ((orbit.distEnd ?? orbit.dist) - orbit.dist) * curve.glide(t);
  const target = new THREE.Vector3(orbit.targetX ?? 0, orbit.targetY ?? 10, orbit.targetZ ?? 0);
  const position = new THREE.Vector3(target.x + d * Math.cos(el) * Math.cos(az), target.y + d * Math.sin(el), target.z + d * Math.cos(el) * Math.sin(az));
  return { position, target };
};

export const CameraRig: React.FC<{ orbit: Orbit }> = ({ orbit }) => {
  const frame = useCurrentFrame();
  const { camera } = useThree();
  const { position, target } = orbitPose(orbit, frame);
  camera.position.copy(position);
  camera.lookAt(target);
  (camera as THREE.PerspectiveCamera).fov = 36;
  (camera as THREE.PerspectiveCamera).near = 1;
  (camera as THREE.PerspectiveCamera).far = 20000;
  camera.updateProjectionMatrix();
  return null;
};

const Model: React.FC<{ scene: THREE.Group; build?: Build; only?: "splats" | "mesh" }> = ({ scene, build, only }) => {
  const frame = useCurrentFrame();

  // Tile nodes are named tile_<n>_<mesh> in export order.
  // Tiles land from the centre outwards, so the place grows as one surface
  // instead of starting as a stray strip. Nodes are named tile_<n>_<mesh>.
  const tiles = useMemo(() => {
    const groups = new Map<number, THREE.Object3D[]>();
    scene.traverse((o) => {
      const m = /^tile_(\d+)_/.exec(o.name);
      if (m && (o as THREE.Mesh).isMesh) {
        const id = Number(m[1]);
        groups.set(id, [...(groups.get(id) ?? []), o]);
      }
    });
    const dist = new Map<number, number>();
    for (const [id, nodes] of groups) {
      const box = new THREE.Box3();
      nodes.forEach((n) => box.expandByObject(n));
      const c = box.getCenter(new THREE.Vector3());
      dist.set(id, Math.hypot(c.x, c.z));
    }
    const order = [...groups.keys()].sort((a, b) => dist.get(a)! - dist.get(b)!);
    const list: { node: THREE.Object3D; rank: number; y: number }[] = [];
    order.forEach((id, i) => {
      for (const node of groups.get(id)!) list.push({ node, rank: i / Math.max(order.length, 1), y: node.position.y });
    });
    return list;
  }, [scene]);

  for (const { node, rank, y } of tiles) {
    node.visible = only !== "splats";
    if (build && node.visible) {
      const appear = build.tilesStart + (build.tilesEnd - build.tilesStart) * rank;
      node.visible = frame >= appear;
      // Each tile drops 10 m and settles in 6 frames.
      node.position.y = y + interpolate(frame, [appear, appear + 6], [10, 0], { ...clamp, easing: curve.land });
    }
  }
  scene.traverse((o) => {
    if (o.name === "splats" && (o as THREE.Mesh).isMesh) {
      const mat = (o as THREE.Mesh).material as THREE.Material;
      o.visible = only !== "mesh";
      if (build) {
        const s = interpolate(frame, [build.splatStart, build.splatEnd], [0, 1], { ...clamp, easing: curve.glide });
        o.visible = o.visible && s > 0;
        mat.opacity = s;
      }
    }
  });
  return <primitive object={scene} />;
};

export const City3D: React.FC<{ file: string; orbit: Orbit; build?: Build; only?: "splats" | "mesh" }> = ({ file, orbit, build, only }) => {
  // Load outside the 3D tree: the canvas mounts asynchronously, too late to
  // hold the frame. The canvas only appears once the model is there.
  const [scene, setScene] = useState<THREE.Group | null>(null);
  const [handle] = useState(() => delayRender(`load ${file}`, { timeoutInMilliseconds: 120000 }));
  useEffect(() => {
    load(file).then((s) => {
      setScene(s.clone(true));
      continueRender(handle);
    });
  }, [file, handle]);
  if (!scene) return null;
  return (
    <ThreeCanvas width={W} height={H} flat gl={{ antialias: true, alpha: true }}>
      <CameraRig orbit={orbit} />
      <Model scene={scene} build={build} only={only} />
    </ThreeCanvas>
  );
};
