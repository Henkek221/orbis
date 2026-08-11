# Sicherheit

## Keine Zugangswerte veröffentlichen

Die lokale `config.json` enthält technische Werte, die nicht in Git, Screenshots,
Issues oder Chat-Nachrichten gehören. Das Repository ignoriert diese Datei
absichtlich. Veröffentliche ausschließlich `config.example.json`.

Falls eine echte `config.json` versehentlich in Git gelandet ist, reicht das
Löschen in einem späteren Commit nicht aus: Die Werte bleiben in der Historie.
Erstelle in diesem Fall eine saubere neue Historie und ersetze betroffene Werte.

## Lokaler Server

Mappy lauscht standardmäßig auf `localhost:8787`. Damit ist die Oberfläche nur
auf dem eigenen Computer erreichbar. Setze `FLYOVER_UI_ADDR` nur dann auf eine
andere Adresse, wenn du die Folgen für dein Netzwerk verstehst.

## Sicherheitsproblem melden

Bitte keine sensiblen Details in einem öffentlichen Issue posten. Teile nur die
minimal nötigen Informationen direkt mit dem Projektinhaber und entferne Tokens,
lokale Pfade und persönliche Daten aus Logs und Screenshots.
