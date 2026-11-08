# Spezifikation: Orbis Launch

## Zweck
- Wofür: Launch-Video für das Open-Source-Werkzeug Orbis (X-Thread, README, Reddit, Show HN)
- Plattform und Format: 16:9, 1920×1080, 30 fps (Hauptversion); 9:16-Schnitt danach
- Länge: ca. 30 s, kein Loop
- Ausgangslage: offen (Fall A, keine Referenz). Formate-Regeln „Launch-Video“ gelten: der Held ist das echte Produkt.

## Text (Entwurf, noch nicht freigegeben)
- Kernsatz: Mit einem Befehl wird ein echter Ort zum 3D-Modell, das jede Engine öffnet.
- Sprechertext: keiner (läuft stumm im Feed). Text im Bild trägt alles.
- Text im Bild, pro Beat:
  1. Befehl im Terminal: `orbis export --place "Brandenburger Tor"`
  2. „One command.“ → „A real place.“ (während die Stadt entsteht)
  3. „Any city with Apple 3D data.“ + Ortsnamen und Koordinaten (echt)
  4. „One file. Every engine.“ + Formatnamen GLB · USDZ · Splats
  5. „Now with Apple's new splat trees.“
  6. `github.com/Henkek221/orbis`

Nacherzähl-Test (Ziel): „Ein Programm, das echte Orte mit einem Befehl in 3D-Modelle für Spiele und Apps verwandelt.“

## Idee
Die Ladeanzeige baut die Stadt: Jede Kachel, die im Terminal als geladen zählt, landet im selben Moment als echtes 3D-Stück im Bild, zuletzt wachsen die Splat-Bäume.

## Marke
- Farben: Hintergrund #000000 / #0b0f1a (Terminal), Text #f5f5f7, Akzent Systemblau #0a84ff (Dark Mode), Grün #30d158 nur für „fertig“
- Schrift: SF Pro Display (Überschriften 600), SF Mono (Terminal)
- Assets: Exporte in `../exports/`, Orbit-Renderer `../tools/orbit`

## Handschrift
1. Das Terminal ist echt: echter Befehl, echte Ausgabezeilen aus dem Export, echte Zahlen.
2. Kacheln landen in der Reihenfolge, in der der Export sie wirklich lädt (Raster x/y, Höhen h aufwärts).
3. Die Kamera umrundet die Stadt durchgehend in einer Richtung, auch über Schnitte hinweg (Mitnehmen).
4. Orte wechseln per Match-Cut auf dem Turm/der Hauptform, nicht per Blende.
5. Bäume kommen als Letztes und weich, sie sind der Höhepunkt.
6. Viel Schwarz, ein Element pro Moment, Apple-Keynote-Ruhe.

## Look
- Bewegungscharakter: ruhig und präzise (Keynote), Kamera gleitet; Kacheln rasten kurz und trocken ein.
- Textur: keine (sauber, digital ist hier das Material).
- Kamera: durchgehender Orbit, langsames Heranfahren; harte Schnitte nur auf Schlag.

## Assets
| Element | Art | Quelle | Status |
|---|---|---|---|
| Stadt-Aufbau Berlin | 3D echt, SceneKit-Render pro Frame, Kacheln schrittweise sichtbar | Orbis-Export Brandenburger Tor | Export da, Render offen |
| Welt-Orbits | 3D echt | Exporte Berlin, NYC, London, Tokyo, Sydney, Dubai, Paris | Exporte laufen |
| Engines | echte Screens | three.js (Browser), Xcode (SceneKit) | offen |
| Splat-Bäume | 3D echt | Berlin-Splats | wartet auf Splat-Semantik |
| Terminal | echte Ausgabe als Text | Orbis-CLI-Log | da |

- Musik: Gratis-Bibliothek, Lizenz prüfen, Tempo ca. 110–120 BPM, ruhig-elektronisch
- Sprache: keine
- Untertitel: nein, Schlüsselwörter stehen groß im Bild (Stumm-Test)
- Soundeffekte: echte Aufnahmen (Freesound CC0 / Mixkit), Varianten: Tastatur (3), Einrasten (3), Luftzug (2), ein tiefer Einschlag

## Sounddesign
| Beat | Geräusch | Variante | Warum (oder: bewusst still) |
|---|---|---|---|
| 1 | echte Tastatur pro Tastendruck, Enter hörbar | 3 Aufnahmen, Tonhöhe ±1 HT | man sieht Tippen |
| 2 | kurzes trockenes Einrasten pro Kachelgruppe, nicht pro Kachel | 3 Varianten | Kacheln landen |
| 2→3 | ein Einschlag auf „A real place.“ | 1 | Höhepunkt Teil 1 |
| 3 | Luftzug auf jedem Ortswechsel, leise | 2 Varianten im Wechsel | Kamera-Ruck |
| 5 | still, Musik trägt | – | Bäume sollen wirken |
| 6 | Enter-Taste | 1 | Schluss-Pointe |

## Kurven
| Bewegungsart | Kurve | Warum |
|---|---|---|
| Kachel landet | schnell rein, 6 Frames, kurz einrasten, kein Federn | mechanisch-präzise |
| Kamera-Orbit | konstante Winkelgeschwindigkeit, Ein-/Ausrollen nur am Anfang/Ende | Keynote-Ruhe |
| Schlagwort | erscheint hart auf dem Schlag | Schrift wird gesetzt |
| Ortsschnitt | harter Schnitt + Kamera-Ruck 4 Frames | Takt |

## Beats
| Zeit | Frames | Was passiert | Komposition | Kamera | Material bzw. Übergang | Ton / Text |
|---|---|---|---|---|---|---|
| 0.0–2.5 s | 0–75 | Terminal, Befehl wird getippt, Enter | nur Schrift, links unten | steht | echtes Terminal | Tastatur |
| 2.5–8.0 s | 75–240 | Fortschritt läuft, Stadt baut sich Kachel für Kachel, Bäume wachsen zuletzt | Terminal klein unten, Stadt groß | Orbit, fährt heran | Kachel-Einrasten | „One command.“ → „A real place.“ |
| 8.0–13.0 s | 240–390 | Weltreise: NYC, Tokyo, London, Dubai, Sydney, Paris | Vollbild, Label unten links | Orbit weiter, Ruck pro Schnitt | Match-Cut auf Hauptform | „Any city with Apple 3D data.“ + Ortsnamen |
| 13.0–19.0 s | 390–570 | Eine Datei fliegt aus dem Terminal in three.js, Xcode | geteilt, Fenster schräg im Raum | steht, Fenster drehen | Mitnehmen der Datei | „One file. Every engine.“ |
| 19.0–25.0 s | 570–750 | Nah an Baumkronen: alt (Seattle) gegen neu (Berlin, Splats) | Detail, zweigeteilt | langsames Seitwärts-Gleiten | Wischer durch Baumkrone | „Now with Apple's new splat trees.“ |
| 25.0–30.0 s | 750–900 | Berlin von oben, Befehl + Repo-Link erscheinen, Enter | weit, Schrift mittig | zieht hoch | Rückkehr zum Anfang | Repo-Link |

## Letzter Frame
Berlin von oben im Dunkel, darüber `orbis export --place "…"` und `github.com/Henkek221/orbis`.

## Ergänzt, nicht vorgegeben
- Englischer Bildtext (globale Zielgruppe), Laufzeit 30 s, keine Stimme.

## Abgenommen
–

## Feedback-Verlauf
- 08.11.2026: Video pausiert, bis die Splat-Bäume sauber sind (Wunsch Henrik).
