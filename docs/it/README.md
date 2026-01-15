# Documentazione OwlMail

Benvenuto nella directory della documentazione OwlMail. Questa directory contiene documentazione tecnica, guide di migrazione e materiali di riferimento API.

## 📚 Struttura della documentazione

### Documenti principali

- **[OwlMail × MailDev - Libro bianco completo su funzionalità, API e migrazione](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)** (Inglese)
  - [中文版本](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.zh-CN.md)
  - Un confronto completo tra OwlMail e MailDev, inclusa la compatibilità API, la parità delle funzionalità e la guida alla migrazione.

### Documentazione interna

- **[Registro di refactoring API](./internal/API_Refactoring_Record.md)** (Inglese)
  - [中文版本](./internal/API_Refactoring_Record.zh-CN.md)
  - Documenta il processo di refactoring API dagli endpoint compatibili con MailDev al nuovo design API RESTful (`/api/v1/`).

## 🌍 Supporto multilingue

Tutti i documenti seguono la convenzione di denominazione: `filename.md` (Inglese, predefinito) e `filename.LANG.md` per altre lingue.

### Lingue supportate

- **English** (`en`) - Predefinito, nessun suffisso di codice lingua
- **简体中文** (`zh-CN`) - Cinese (Semplificato)
- **Italiano** (`it`) - Italiano

### Formato del codice lingua

I codici lingua seguono lo standard [ISO 639-1](https://en.wikipedia.org/wiki/ISO_639-1):
- `zh-CN` - Cinese (Semplificato)
- `de` - Tedesco (futuro)
- `fr` - Francese (futuro)
- `it` - Italiano
- `ja` - Giapponese (futuro)
- `ko` - Coreano (futuro)

## 📖 Come leggere la documentazione

1. **Lingua predefinita**: I documenti senza suffisso di codice lingua sono in inglese (predefinito).
2. **Altre lingue**: I documenti con suffisso di codice lingua (ad es. `.zh-CN.md`) sono traduzioni.
3. **Struttura delle directory**: I documenti sono organizzati per argomento, con documentazione interna nella sottodirectory `internal/`.

## 🔄 Contribuire

Quando si aggiunge nuova documentazione:

1. Creare prima la versione inglese (predefinita, nessun codice lingua).
2. Aggiungere traduzioni con il suffisso di codice lingua appropriato.
3. Aggiornare questo README per includere collegamenti ai nuovi documenti.
4. Seguire le convenzioni di denominazione esistenti.

## 📝 Categorie di documenti

- **Guide di migrazione**: Aiutano gli utenti a migrare da MailDev a OwlMail
- **Documentazione API**: Riferimento tecnico API e registri di refactoring
- **Documentazione interna**: Note di sviluppo e processi interni

---

Per ulteriori informazioni su OwlMail, visitare il [README principale](../README.it.md).
