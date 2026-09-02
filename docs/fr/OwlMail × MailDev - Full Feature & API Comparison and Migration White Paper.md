# OwlMail × MailDev × MailCatcher : Livre blanc complet sur les fonctionnalités, l'API et la migration

> **Une comparaison approfondie au niveau du code source + Guide de migration pour les utilisateurs et les développeurs**

> ⚠️ **Document en cours de traduction**
> 
> Cette traduction est en cours. Pour l'instant, veuillez consulter la [version anglaise](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) ou d'autres versions disponibles :
> - [English](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
> - [简体中文](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
> 
> **Contributions bienvenues** : Si vous souhaitez contribuer à cette traduction, veuillez consulter le [guide de contribution](../../.github/CONTRIBUTING.md).

---

## 📋 Résumé exécutif

OwlMail, MailDev et MailCatcher couvrent les mêmes workflows de développement essentiels,
mais ils ne sont **ni identiques au niveau du protocole, ni interchangeables sans
validation**. Les préfixes API, les réponses, l'état de lecture et le protocole
temps réel diffèrent. Consultez la [référence API](../en/API-Reference.md).

OwlMail fournit, uniquement sur la branche main examinée (pas dans la v0.6.0), un point de terminaison MCP en lecture seule, désactivé par défaut. MailDev 3 propose un workflow MCP plus large ; MailCatcher n'intègre pas MCP. L'option `-maildev-rest-compat`, désactivée par défaut, expose les routes REST
MailDev actuelles sous `/api`. Elle n'ajoute aucune compatibilité Socket.IO.

> **Note** : Le contenu complet sera disponible une fois la traduction terminée. En attendant, veuillez vous référer à la version anglaise pour les détails complets.

---

## Comment contribuer

Si vous souhaitez aider à traduire ce document :

1. Fork le dépôt
2. Créez une branche pour votre traduction
3. Traduisez le contenu de [OwlMail × MailDev - Full Feature & API Comparison and Migration White Paper.md](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
4. Soumettez une Pull Request

Pour plus d'informations, consultez le [guide de contribution](../../.github/CONTRIBUTING.md).
