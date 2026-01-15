# Politique de Sécurité

## Versions Prise en Charge

Nous fournissons actuellement des mises à jour de sécurité pour les versions suivantes :

| Version | Prise en charge |
|---------|-----------------|
| Dernière | ✅ Oui |
| Version majeure précédente | ✅ Oui |
| Versions plus anciennes | ❌ Non |

## Signaler une Vulnérabilité

Nous prenons la sécurité d'OwlMail au sérieux. Si vous découvrez une vulnérabilité de sécurité, veuillez **ne pas** la signaler dans un problème public.

### Comment Signaler

Veuillez signaler les vulnérabilités de sécurité :

1. **Email** : Envoyer à [security@owlmail.dev](mailto:security@owlmail.dev)
   - Veuillez utiliser un objet descriptif
   - Inclure une description détaillée de la vulnérabilité
   - Fournir des étapes pour reproduire (si possible)
   - Expliquer l'impact potentiel

2. **Attendre une Réponse** : Nous accuserons réception dans les 48 heures

3. **Processus** :
   - Nous évaluerons la gravité de la vulnérabilité
   - Si confirmée comme problème de sécurité, nous :
     - Développerons un correctif
     - Préparerons un avis de sécurité
     - Publierons une version corrigée
   - Nous vous tiendrons informé de l'avancement

### Ce qu'il Faut Inclure

Pour nous aider à mieux comprendre et corriger la vulnérabilité, veuillez inclure dans votre rapport :

- **Type de Vulnérabilité** : par ex. injection SQL, XSS, escalade de privilèges, etc.
- **Composant Affecté** : Quelle fonctionnalité ou composant est affecté
- **Étapes pour Reproduire** : Étapes détaillées sur la façon de reproduire la vulnérabilité
- **Impact Potentiel** : Quelles conséquences la vulnérabilité pourrait avoir
- **Correctif Suggéré** (le cas échéant)

### Bug Bounty

Bien que nous n'ayons pas actuellement de programme formel de bug bounty, nous prenons les contributions de sécurité au sérieux et les reconnaîtrons de manière appropriée (avec votre permission).

## Meilleures Pratiques de Sécurité

### Pour les Utilisateurs

- **Rester à Jour** : Gardez OwlMail à jour avec la dernière version
- **Sécurité Réseau** : Utilisez HTTPS/TLS dans les environnements de production
- **Contrôle d'Accès** : Configurez une authentification et une autorisation appropriées
- **Isolation de l'Environnement** : N'exposez pas d'instances non protégées sur des réseaux publics
- **Informations Sensibles** : Ne codez pas en dur les mots de passe ou les clés dans le code ou la configuration

### Pour les Développeurs

- **Mises à Jour des Dépendances** : Mettez régulièrement à jour les dépendances pour obtenir les correctifs de sécurité
- **Revue de Code** : Examinez attentivement tous les changements de code
- **Tests de Sécurité** : Effectuez des tests de sécurité pendant le développement
- **Privilèges Minimaux** : Suivez le principe du moindre privilège
- **Validation des Entrées** : Validez et nettoyez toujours les entrées utilisateur

## Problèmes de Sécurité Connus

Nous divulguerons les problèmes de sécurité connus après leur correction. Consultez [Security Advisories](https://github.com/soulteary/owlmail/security/advisories) pour plus de détails.

## Mises à Jour de Sécurité

Les mises à jour de sécurité seront publiées via :

- GitHub Releases
- Security Advisories
- Mises à jour de la documentation du projet

## Contact

- **Problèmes de Sécurité** : [security@owlmail.dev](mailto:security@owlmail.dev)
- **Problèmes Généraux** : Soumettre dans [GitHub Issues](https://github.com/soulteary/owlmail/issues)

## Remerciements

Nous apprécions tous les chercheurs et utilisateurs qui signalent de manière responsable les problèmes de sécurité. Vos contributions nous aident à garder OwlMail sécurisé.
