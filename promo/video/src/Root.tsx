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
