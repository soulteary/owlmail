# Politica di Sicurezza

## Versioni Supportate

Attualmente forniamo aggiornamenti di sicurezza per le seguenti versioni:

| Versione | Supportata |
|----------|------------|
| Ultima | ✅ Sì |
| Versione principale precedente | ✅ Sì |
| Versioni più vecchie | ❌ No |

## Segnalazione di una Vulnerabilità

Prendiamo sul serio la sicurezza di OwlMail. Se scopri una vulnerabilità di sicurezza, per favore **non** segnalarla in un issue pubblico.

### Come Segnalare

Per favore segnala le vulnerabilità di sicurezza:

1. **Email**: Invia a [security@owlmail.dev](mailto:security@owlmail.dev)
   - Per favore usa una riga oggetto descrittiva
   - Includi una descrizione dettagliata della vulnerabilità
   - Fornisci passaggi per riprodurre (se possibile)
   - Spiega l'impatto potenziale

2. **Attendi Risposta**: Confermeremo la ricezione entro 48 ore

3. **Processo**:
   - Valuteremo la gravità della vulnerabilità
   - Se confermata come problema di sicurezza, faremo:
     - Sviluppare una correzione
     - Preparare un advisory di sicurezza
     - Rilasciare una versione corretta
   - Ti terremo aggiornato sui progressi

### Cosa Includere

Per aiutarci a capire e correggere meglio la vulnerabilità, per favore includi nel tuo report:

- **Tipo di Vulnerabilità**: es. iniezione SQL, XSS, escalation dei privilegi, ecc.
- **Componente Affetto**: Quale funzionalità o componente è affetto
- **Passaggi per Riprodurre**: Passaggi dettagliati su come riprodurre la vulnerabilità
- **Impatto Potenziale**: Quali conseguenze potrebbe avere la vulnerabilità
- **Correzione Suggerita** (se presente)

### Bug Bounty

Anche se attualmente non abbiamo un programma formale di bug bounty, prendiamo sul serio i contributi alla sicurezza e li riconosceremo appropriatamente (con il tuo permesso).

## Best Practice di Sicurezza

### Per gli Utenti

- **Mantieni Aggiornato**: Mantieni OwlMail aggiornato all'ultima versione
- **Sicurezza di Rete**: Usa HTTPS/TLS negli ambienti di produzione
- **Controllo Accessi**: Configura autenticazione e autorizzazione appropriate
- **Isolamento Ambiente**: Non esporre istanze non protette su reti pubbliche
- **Informazioni Sensibili**: Non hardcodare password o chiavi nel codice o nella configurazione

### Per gli Sviluppatori

- **Aggiornamenti Dipendenze**: Aggiorna regolarmente le dipendenze per ottenere patch di sicurezza
- **Revisione Codice**: Rivedi attentamente tutte le modifiche al codice
- **Test di Sicurezza**: Esegui test di sicurezza durante lo sviluppo
- **Privilegi Minimi**: Segui il principio dei privilegi minimi
- **Validazione Input**: Valida e sanifica sempre l'input dell'utente

## Problemi di Sicurezza Conosciuti

Riveleremo problemi di sicurezza conosciuti dopo che sono stati corretti. Controlla [Security Advisories](https://github.com/soulteary/owlmail/security/advisories) per i dettagli.

## Aggiornamenti di Sicurezza

Gli aggiornamenti di sicurezza saranno rilasciati tramite:

- GitHub Releases
- Security Advisories
- Aggiornamenti documentazione progetto

## Contatto

- **Problemi di Sicurezza**: [security@owlmail.dev](mailto:security@owlmail.dev)
- **Problemi Generali**: Invia in [GitHub Issues](https://github.com/soulteary/owlmail/issues)

## Ringraziamenti

Apprezziamo tutti i ricercatori e gli utenti che segnalano responsabilmente problemi di sicurezza. I tuoi contributi ci aiutano a mantenere OwlMail sicuro.
