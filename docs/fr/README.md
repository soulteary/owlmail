# Documentation OwlMail

Bienvenue dans le répertoire de documentation OwlMail. Ce répertoire contient de la documentation technique, des guides de migration et des matériaux de référence API.

## 📸 Aperçu

![Aperçu OwlMail](../../.github/assets/preview.png)

## 🎥 Vidéo de démonstration

![Vidéo de démonstration](../../.github/assets/realtime.gif)

## 📚 Structure de la documentation

### Documents principaux

- **[Transfert Webhook](../en/Webhook-Forwarding.md)** (Anglais, [中文](../zh-CN/Webhook-Forwarding.md))
  - Configurer les règles, les messages personnalisés, les signatures HMAC et l'intégration avec `soulteary/webhook`.

- **[OwlMail × MailDev - Livre blanc complet sur les fonctionnalités, l'API et la migration](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)** (Anglais)
  - [中文版本](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
  - Une comparaison complète entre OwlMail et MailDev, incluant la compatibilité API, la parité des fonctionnalités et le guide de migration.

### Documentation interne

- **[Enregistrement de refactorisation API](./internal/API_Refactoring_Record.md)** (Anglais)
  - [中文版本](../zh-CN/internal/API_Refactoring_Record.md)
  - Documente le processus de refactorisation API des points de terminaison compatibles MailDev vers la nouvelle conception API RESTful (`/api/v1/`).

## 🌍 Support multilingue

Tous les documents suivent la convention de nommage : `filename.md` (Anglais, par défaut) et `filename.LANG.md` pour les autres langues.

### Langues supportées

- **English** (`en`) - Par défaut, sans suffixe de code de langue
- **简体中文** (`zh-CN`) - Chinois (Simplifié)
- **Français** (`fr`) - Français

### Format du code de langue

Les codes de langue suivent la norme [ISO 639-1](https://en.wikipedia.org/wiki/ISO_639-1) :
- `zh-CN` - Chinois (Simplifié)
- `de` - Allemand (à venir)
- `fr` - Français
- `it` - Italien (à venir)
- `ja` - Japonais (à venir)
- `ko` - Coréen (à venir)

## 📖 Comment lire la documentation

1. **Langue par défaut** : Les documents sans suffixe de code de langue sont en anglais (par défaut).
2. **Autres langues** : Les documents avec un suffixe de code de langue (par ex. `.zh-CN.md`) sont des traductions.
3. **Structure des répertoires** : Les documents sont organisés par sujet, la documentation interne se trouve dans le sous-répertoire `internal/`.

## 🔄 Contribution

Lors de l'ajout de nouvelle documentation :

1. Créez d'abord la version anglaise (par défaut, sans code de langue).
2. Ajoutez des traductions avec le suffixe de code de langue approprié.
3. Mettez à jour ce README pour inclure des liens vers les nouveaux documents.
4. Suivez les conventions de nommage existantes.

## 📝 Catégories de documents

- **Guides de migration** : Aident les utilisateurs à migrer de MailDev vers OwlMail
- **Documentation API** : Référence technique API et enregistrements de refactorisation
- **Documentation interne** : Notes de développement et processus internes

---

Pour plus d'informations sur OwlMail, veuillez visiter le [README principal](../../README.fr.md).
