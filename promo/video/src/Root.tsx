// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

import { Composition } from "remotion";
import { MyComposition } from "./Composition";
import { LIGHT_TOTAL, Light } from "./Light";
import { FPS, H, W } from "./theme";

export const RemotionRoot: React.FC = () => {
  return (
    <>
      <MyComposition />
      <Composition id="OrbisLight" component={Light} durationInFrames={LIGHT_TOTAL} fps={FPS} width={W} height={H} />
    </>
  );
};
