# Credits und Herkunft

Orbis ist eine eigenständige Benutzeroberfläche und ein lokaler Export-Workflow.
Das Projekt wäre ohne die technische Vorarbeit anderer nicht möglich.

## Technische Grundlage

### retroplasma/flyover-reverse-engineering

[retroplasma/flyover-reverse-engineering](https://github.com/retroplasma/flyover-reverse-engineering)
enthält die ursprüngliche Reverse-Engineering-Arbeit zu Flyover-Kacheln,
Manifesten, Authentifizierung, C3M/C3MM-Dateien, Mesh-Dekompression und
OBJ-Export. Diese Arbeit bildet den Kern der übernommenen Decoder- und
Abruflogik.

### christiangda/flyover-reverse-engineering

[christiangda/flyover-reverse-engineering](https://github.com/christiangda/flyover-reverse-engineering)
hat die ursprüngliche Arbeit für neuere Go- und Protobuf-Versionen modernisiert.
Der Go-Modulpfad lautet inzwischen `github.com/Henkek221/orbis`.

## Oberfläche und Assets

- [Leaflet](https://leafletjs.com/) stellt die interaktive Karte bereit.
- [OpenStreetMap-Mitwirkende](https://www.openstreetmap.org/copyright) liefern
  die sichtbaren Basiskartendaten.
- Das kleine Fahrzeug im Viewer stammt aus den Khronos-glTF-Beispielassets. Die
  zugehörige CC0-Erklärung liegt in
  [`internal/server/static/assets/TOY-CAR-LICENSE.txt`](internal/server/static/assets/TOY-CAR-LICENSE.txt).

## Was in Orbis neu hinzugekommen ist

- lokale Browser-Oberfläche mit Ortssuche und Kartenauswahl
- sichtbarer, ehrlicher Status für bekannte und unbekannte 3D-Abdeckung
- Flächenexport mit Fortschritt, Diagnosewerten und verständlichen Namen
- eingebetteter 3D-Viewer mit OBJ-Drag-and-drop
- Lauf- und Fahrmodus, Licht, Tageszeit und Darstellungsoptionen
- lokale Wiederherstellung abgeschlossener Exporte
- Ein-Klick-Builds für macOS, Windows und Linux
- Kommandozeile `orbis` und Go-Bibliothek `pkg/orbis`
- GLB- und USD(Z)-Export für Game-Engines und Apple-Plattformen
- Analyse von Apples neuem 3D-Format R3D v2 (Container, Index, Texturen)

## Rechtlicher Hinweis

Die ursprünglichen Repositories weisen derzeit keine verlässliche
`LICENSE`-Datei aus. Eine Nennung der Urheber ist kein Ersatz für eine Lizenz.
Orbis vergibt deshalb keine Lizenz für die geerbten Dateien; welche das sind,
steht in [`LICENSING.md`](LICENSING.md). Der eigene Orbis-Code steht unter der
Apache-2.0-Lizenz mit Pflicht zur Namensnennung über [`NOTICE`](NOTICE).

Apple, Apple Maps und Flyover sind Marken beziehungsweise Dienste von Apple Inc.
Orbis ist nicht mit Apple verbunden. Für Karten-, 3D- und Exportdaten können
zusätzliche Bedingungen gelten.
