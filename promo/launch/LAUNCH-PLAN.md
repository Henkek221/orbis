# Orbis – Launch-Plan

Stand: 8.11.2026. Der R3D-Decoder ist an echten Kacheln bestätigt (Berlin,
New York, Tokio, Sydney; Paris gegen klassisches C3M). Dubai hat keine 3D-Daten.

## Was wir launchen

**Orbis**: ein Open-Source-Werkzeug (CLI, Go-Bibliothek, Browser-UI), das echte
Orte als texturierte 3D-Modelle exportiert, aus Apples 3D-Kartendaten, lokal
auf einem Mac. Formate: GLB (jede Engine), USDZ (Xcode, RealityKit, visionOS),
OBJ, Minecraft.

**Ein Satz:** „Turn real places into game-ready 3D models with one command.“

**Für wen:** Spieleentwickler, die echte Spielwelten bauen (Unity, Unreal,
Godot), Blender-Leute, three.js-Entwickler, visionOS/AR-Entwickler,
Architektur- und Stadtvisualisierung, Photogrammetrie-Nerds.

**Warum jetzt:** Apple hat mit iOS 27 neue 3D-Daten ausgerollt, inklusive
eigener „Splats“ für Bäume. Bisher kam niemand außerhalb von Apple
Maps an diese Daten.

## Kanäle (ORB)

| Kanal | Art | Zweck |
| --- | --- | --- |
| GitHub-Repo `Henkek221/orbis` | eigen | Ziel aller Links: README mit Video, Quick Start, Releases, Stars |
| Reddit | gemietet | Hauptreichweite bei Entwicklern, siehe `reddit.md` |
| X / Twitter | gemietet | Video-Thread, siehe `twitter.md` |
| Hacker News | gemietet | Show HN, siehe `hn.md` |
| kvasir-Repo | eigen | Querverweis, Link auf Orbis aktualisieren |
| Creator (gameDev-YouTube, Blender-Leute) | geliehen | später: ein Export-Beispiel an 3–5 Creator schicken |

## Reihenfolge

1. **Vorher (T-2 Tage)**
   - Repo öffentlich mit fertigem README, Release-ZIPs (macOS/Windows/Linux), Hero-Video oben.
   - 3 Beispiel-Renders und 1 GIF pro Engine (Unity, Godot, Blender, Xcode) als Beweis.
   - GitHub-Topics setzen: `3d`, `photogrammetry`, `gltf`, `usdz`, `gamedev`, `cli`, `golang`, `maps`, `unity`, `godot`, `blender`, `visionos`.
2. **Launch-Tag (Dienstag oder Mittwoch, 15:00 MEZ = 9:00 US-Ostküste)**
   - 15:00 Show HN posten.
   - 15:15 X-Thread mit Video.
   - 15:30–18:00 Reddit, gestaffelt, ein Sub pro halbe Stunde, jeweils angepasster Text. Nicht in alle gleichzeitig (Spam-Filter).
   - Den ganzen Abend jede Frage beantworten. Fragen sind der Algorithmus.
3. **Danach**
   - Häufige Fragen ins README (FAQ).
   - Nach einer Woche: „Was die Community damit gebaut hat“-Post mit den besten Nutzer-Exporten (Einverständnis einholen).

## Risiken, ehrlich

- **Apples Nutzungsbedingungen** verbieten das Extrahieren und Scrapen von Kartendaten. Takedowns gegen das seit 2019 öffentliche Vorgänger-Projekt (627 Sterne) sind nicht bekannt, aber ein lauter Launch macht sichtbar. Text überall: lokal, Forschung, Prototyping, Daten nicht weiterverbreiten.
- **Lizenz:** Eigener Orbis-Code ist Apache-2.0 mit Pflicht zur Namensnennung („Based on Orbis by Henrik Hellpap“ über NOTICE, der Name bleibt auch bei Umbenennung). 28 geerbte Upstream-Dateien haben keine Lizenz und sind ausgenommen (LICENSING.md). Also: „Apache-2.0, credit required“ sagen, nicht „100 % open source“. Wer fragt: ehrlich sagen.
- **Erwartung „ganze Erde“:** Nur sagen, was stimmt: 1.434 Regionen (56 klassisch, 1.378 R3D). Nicht „jeder Ort der Erde“ (Dubai z. B. hat keine 3D-Daten).
- **Keine Apple-Marken als Branding:** Kein Apple-Logo, kein „Apple Maps Exporter“ im Namen. „Uses Apple's 3D map data“ als beschreibende Aussage ist ok.

## Assets-Checkliste

- [x] Hero-Video 16:9, 29 s (X, README, HN-Link)
- [ ] Hochkant-Schnitt 9:16, 20 s (X/Reddit mobil)
- [x] Showcase-Exporte: Space Needle, Mailänder Dom, Notre-Dame, Arc de Triomphe, Canada Place, Palacio Real
- [ ] Engine-Beweise: Screenshot GLB in Godot/Blender/three.js, USDZ in Xcode
- [ ] README mit Video, Quick Start, Formattabelle, FAQ
- [ ] Release v0.2.0 mit ZIPs
