# Orbis promo

Everything used to make the launch film and the launch posts.

| Folder | What |
| --- | --- |
| `video/` | Remotion project. `OrbisLight` is the current launch film (light, card-based style); `OrbisLaunch` is the earlier dark terminal cut. |
| `launch/` | Launch plan and drafts for Reddit, X and Hacker News. Internal notes, review before the repo goes public. |
| `tools/` | Small Swift helpers used to render turntables and find window IDs for screen recordings. |
| `webview/` | Local three.js / Gaussian splat pages used to check exports and take the engine screenshots. |

## Render the film

The 3D models and the music are not in the repo: the models are exports of
Apple's map data, and the Mixkit licence does not allow passing the tracks on
as files.

1. Export the models with Orbis into `video/public/models/` (`berlin.glb`,
   `tokyo-tower.glb`, `new-york-empire-state.glb`, `paris-eiffel.glb`,
   `sydney-opera-house.glb`; for the dark cut also `berlin-fernsehturm.glb`).
   For example: `orbis export --place "Tokyo Tower" --format glb`.
2. Download "Smooth Grooves" by Ahjay Stelino from Mixkit
   (`https://assets.mixkit.co/music/101/101.mp3`) to
   `video/public/audio/music-smooth.mp3` and build the bed with a muffled intro:

   ```bash
   cd video/public/audio
   ffmpeg -ss 0.726 -t 50 -i music-smooth.mp3 -filter_complex "[0:a]asplit[a][b];[a]atrim=0:5.55,lowpass=f=500,volume=1.6,asetpts=PTS-STARTPTS[lo];[b]atrim=5.25,asetpts=PTS-STARTPTS[hi];[lo][hi]acrossfade=d=0.3:c1=tri:c2=tri" -ar 44100 music-bed.wav
   ```

3. Render and normalise:

   ```bash
   cd video
   npm i
   npx remotion render OrbisLight out/orbis-light.mp4 --gl=angle
   ffmpeg -i out/orbis-light.mp4 -c:v copy -af loudnorm=I=-14:TP=-1.5:LRA=11 -c:a aac -b:a 192k out/orbis-launch.mp4
   ```

Sound effects are CC0 recordings from Freesound; sources in
`video/public/audio/LICENSES.md`.
