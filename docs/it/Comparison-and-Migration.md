# OwlMail × MailDev × MailCatcher: Libro bianco completo su funzionalità, API e migrazione

> **Un confronto approfondito a livello di codice sorgente + Guida alla migrazione per utenti e sviluppatori**

> ⚠️ **Documento in fase di traduzione**
> 
> Questa traduzione è in corso. Per ora, si prega di consultare la [versione inglese](../en/Comparison-and-Migration.md) o altre versioni disponibili:
> - [English](../en/Comparison-and-Migration.md)
> - [简体中文](../zh-CN/Comparison-and-Migration.md)
> 
> **Contributi benvenuti**: Se desideri contribuire a questa traduzione, consulta la [guida al contributo](../../.github/CONTRIBUTING.md).

---

## 📋 Executive Summary

OwlMail, MailDev e MailCatcher coprono gli stessi flussi di sviluppo fondamentali, ma **non
sono equivalenti a livello di protocollo né intercambiabili senza verifica**.
Prefissi API, risposte, stato di lettura e protocollo in tempo reale differiscono.
Consulta il [riferimento API](../en/API-Reference.md).

OwlMail 0.8.0 include interfacce MCP di sola lettura, disattivate per impostazione predefinita, tramite Streamable HTTP e stdio. MailDev 3 offre un flusso MCP più ampio; MailCatcher non include MCP. L'opzione `-maildev-rest-compat`, disattivata per impostazione predefinita,
espone le route REST MailDev correnti sotto `/api`. Non abilita Socket.IO.

> **Nota**: Il contenuto completo sarà disponibile una volta completata la traduzione. Nel frattempo, si prega di fare riferimento alla versione inglese per i dettagli completi.

---

## Come contribuire

Se desideri aiutare a tradurre questo documento:

1. Fai un fork del repository
2. Crea un branch per la tua traduzione
3. Traduci il contenuto da [Comparison-and-Migration.md](../en/Comparison-and-Migration.md)
4. Invia una Pull Request

Per ulteriori informazioni, consulta la [guida al contributo](../../.github/CONTRIBUTING.md).
