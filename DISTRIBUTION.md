# Mappy – Kurzanleitung für die Testversion

Mappy läuft lokal auf deinem Computer und öffnet seine Oberfläche im Browser.
Du musst nichts installieren.

## Vor dem ersten Start

Auf macOS ist keine Konfigurationsdatei nötig. Mappy erkennt die benötigte
Apple-Systemkonfiguration beim ersten Export automatisch.

Unter Windows und Linux legst du die separat erhaltene Datei `config.json`
direkt neben das Programm:

```text
Mappy-Test/
├── Mappy.exe
└── config.json
```

Unter Windows heißt das Programm `Mappy.exe`, unter Linux `Mappy`.

Wichtig: Die `config.json` enthält private technische Zugangswerte. Nicht auf
GitHub, in Chats oder in einem Issue veröffentlichen.

## Starten

- **macOS:** Rechtsklick auf `Mappy.app` → **Öffnen** → **Öffnen**.
- **Windows:** `MAPPY STARTEN.cmd` doppelklicken. Bei einer Warnung **Weitere
  Informationen** → **Trotzdem ausführen** wählen.
- **Linux:** `MAPPY STARTEN.sh` ausführen. Das Skript setzt die nötigen Rechte
  selbst und startet Mappy.

Wenn unter Windows oder Linux noch keine Konfiguration vorhanden ist, zeigt
Mappy beim Start einen einzigen Import-Schritt. `config.json` auswählen oder in
das Fenster ziehen; danach wird sie lokal gespeichert und beim nächsten Start
automatisch verwendet.

Die Testversion ist noch nicht von Apple notarisiert und besitzt kein
Windows-Code-Signing-Zertifikat. Deshalb können macOS und Windows beim ersten
Start warnen.

Mappy öffnet automatisch `http://localhost:8787`. Das gestartete Programm muss
während der Nutzung geöffnet bleiben. Exporte und Zwischendaten werden im
gleichen Ordner gespeichert.

## Testablauf

1. Einen Ort suchen.
2. Einen kleinen Bereich auf der Karte markieren.
3. Export starten und warten, bis er fertig ist.
4. Das Modell direkt im Viewer öffnen.
5. Viewer, Drag-and-drop und Fahrmodus ausprobieren.

Nicht jeder Ort besitzt 3D-Daten. Starte am besten mit einer bekannten größeren
Stadt und einer Fläche von ungefähr 150–300 Metern.

## Wenn etwas nicht funktioniert

Für eine Fehlermeldung bitte Betriebssystem, Ort, ungefähre Flächengröße und
einen Screenshot notieren. Niemals die `config.json` mitsenden.
