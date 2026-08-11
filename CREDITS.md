# Credits und Herkunft

Mappy ist eine eigenständige Benutzeroberfläche und ein lokaler Export-Workflow.
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
Der Go-Modulpfad im aktuellen Quellcode verweist aus Kompatibilitätsgründen noch
auf dieses Repository.

## Oberfläche und Assets

- [Leaflet](https://leafletjs.com/) stellt die interaktive Karte bereit.
- [OpenStreetMap-Mitwirkende](https://www.openstreetmap.org/copyright) liefern
  die sichtbaren Basiskartendaten.
- Das kleine Fahrzeug im Viewer stammt aus den Khronos-glTF-Beispielassets. Die
  zugehörige CC0-Erklärung liegt in
  [`cmd/export-ui/static/assets/TOY-CAR-LICENSE.txt`](cmd/export-ui/static/assets/TOY-CAR-LICENSE.txt).

## Was in Mappy neu hinzugekommen ist

- lokale Browser-Oberfläche mit Ortssuche und Kartenauswahl
- sichtbarer, ehrlicher Status für bekannte und unbekannte 3D-Abdeckung
- Flächenexport mit Fortschritt, Diagnosewerten und verständlichen Namen
- eingebetteter 3D-Viewer mit OBJ-Drag-and-drop
- Lauf- und Fahrmodus, Licht, Tageszeit und Darstellungsoptionen
- lokale Wiederherstellung abgeschlossener Exporte
- Ein-Klick-Builds für macOS, Windows und Linux

## Rechtlicher Hinweis

Die ursprünglichen Repositories weisen derzeit keine verlässliche
`LICENSE`-Datei aus. Eine Nennung der Urheber ist kein Ersatz für eine Lizenz.
Mappy vergibt deshalb keine pauschale neue Lizenz für geerbte Bestandteile.

Apple, Apple Maps und Flyover sind Marken beziehungsweise Dienste von Apple Inc.
Mappy ist nicht mit Apple verbunden. Für Karten-, 3D- und Exportdaten können
zusätzliche Bedingungen gelten.
