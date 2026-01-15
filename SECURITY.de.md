# Sicherheitsrichtlinie

## Unterstützte Versionen

Wir bieten derzeit Sicherheitsupdates für die folgenden Versionen:

| Version | Unterstützt |
|---------|-------------|
| Neueste | ✅ Ja |
| Vorherige Hauptversion | ✅ Ja |
| Ältere Versionen | ❌ Nein |

## Meldung einer Sicherheitslücke

Wir nehmen die Sicherheit von OwlMail ernst. Wenn Sie eine Sicherheitslücke entdecken, melden Sie diese bitte **nicht** in einem öffentlichen Issue.

### Wie man meldet

Bitte melden Sie Sicherheitslücken:

1. **E-Mail**: Senden Sie an [security@owlmail.dev](mailto:security@owlmail.dev)
   - Bitte verwenden Sie eine beschreibende Betreffzeile
   - Fügen Sie eine detaillierte Beschreibung der Sicherheitslücke bei
   - Stellen Sie Schritte zur Reproduktion bereit (wenn möglich)
   - Erklären Sie die potenzielle Auswirkung

2. **Warten auf Antwort**: Wir werden den Eingang innerhalb von 48 Stunden bestätigen

3. **Prozess**:
   - Wir werden den Schweregrad der Sicherheitslücke bewerten
   - Wenn als Sicherheitsproblem bestätigt, werden wir:
     - Einen Fix entwickeln
     - Eine Sicherheitsempfehlung vorbereiten
     - Eine gepatchte Version veröffentlichen
   - Wir werden Sie über den Fortschritt auf dem Laufenden halten

### Was enthalten sein sollte

Um uns zu helfen, die Sicherheitslücke besser zu verstehen und zu beheben, fügen Sie bitte in Ihrem Bericht bei:

- **Art der Sicherheitslücke**: z.B. SQL-Injection, XSS, Rechteeskalation usw.
- **Betroffene Komponente**: Welche Funktion oder Komponente betroffen ist
- **Schritte zur Reproduktion**: Detaillierte Schritte zur Reproduktion der Sicherheitslücke
- **Potenzielle Auswirkung**: Welche Konsequenzen die Sicherheitslücke haben könnte
- **Vorgeschlagener Fix** (falls vorhanden)

### Bug Bounty

Während wir derzeit kein formelles Bug-Bounty-Programm haben, nehmen wir Sicherheitsbeiträge ernst und werden sie angemessen würdigen (mit Ihrer Erlaubnis).

## Sicherheitsbest Practices

### Für Benutzer

- **Aktualisiert bleiben**: Halten Sie OwlMail auf dem neuesten Stand
- **Netzwerksicherheit**: Verwenden Sie HTTPS/TLS in Produktionsumgebungen
- **Zugriffskontrolle**: Konfigurieren Sie angemessene Authentifizierung und Autorisierung
- **Umgebungsisolation**: Setzen Sie keine ungeschützten Instanzen in öffentlichen Netzwerken aus
- **Sensible Informationen**: Speichern Sie keine Passwörter oder Schlüssel im Code oder in der Konfiguration

### Für Entwickler

- **Abhängigkeitsupdates**: Aktualisieren Sie regelmäßig Abhängigkeiten, um Sicherheitspatches zu erhalten
- **Code-Review**: Überprüfen Sie alle Codeänderungen sorgfältig
- **Sicherheitstests**: Führen Sie Sicherheitstests während der Entwicklung durch
- **Minimale Rechte**: Befolgen Sie das Prinzip der minimalen Rechte
- **Eingabevalidierung**: Validieren und bereinigen Sie immer Benutzereingaben

## Bekannte Sicherheitsprobleme

Wir werden bekannte Sicherheitsprobleme nach ihrer Behebung offenlegen. Überprüfen Sie [Security Advisories](https://github.com/soulteary/owlmail/security/advisories) für Details.

## Sicherheitsupdates

Sicherheitsupdates werden veröffentlicht über:

- GitHub Releases
- Security Advisories
- Projekt-Dokumentationsupdates

## Kontakt

- **Sicherheitsprobleme**: [security@owlmail.dev](mailto:security@owlmail.dev)
- **Allgemeine Probleme**: Einreichen in [GitHub Issues](https://github.com/soulteary/owlmail/issues)

## Danksagungen

Wir schätzen alle Forscher und Benutzer, die verantwortungsvoll Sicherheitsprobleme melden. Ihre Beiträge helfen uns, OwlMail sicher zu halten.
