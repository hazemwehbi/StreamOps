# Mesure de référence locale du 25 octobre 2026

Ce résultat provient d’un essai fonctionnel sur un poste Windows. Il ne constitue ni une capacité de production ni un engagement de service. L’essai utilise l’outil de génération et le scénario k6 avec PostgreSQL réel. Il s’agit d’un essai court unique, sans préchauffe chronométrée distincte ni maîtrise de la charge des autres applications.

| Paramètre | Valeur |
| --- | --- |
| Date et heure | 25 octobre 2026 vers 11 h 47, Europe/Paris |
| Système | Microsoft Windows 10.0.19045 x64 |
| Processeur | Intel64 Family 6 Model 142 Stepping 11, GenuineIntel ; 8 processeurs logiques visibles |
| Mémoire totale disponible signalée à .NET | 17 061 695 488 octets |
| Outils | Go 1.27.3, PostgreSQL 17.11 et k6 v2.3.0 |
| Topologie | API, base et client sur le même poste ; interface locale IPv4 ; HTTP sans TLS ; sans Docker |
| API | Un binaire statique ; 10 connexions SQL au maximum ; limite de 100 requêtes/s avec capacité de rafale de 200 ; journaux JSON actifs ; aucun export OTLP |
| Données | 1 000 services synthétiques en staging, aucun incident ni contrôle de santé |
| PostgreSQL | Configuration initdb locale par défaut, dont shared_buffers=128MB ; aucune optimisation |
| Requête | GET authentifié /v1/services?limit=20 avec calcul du total et lecture des lignes |
| Scénario | Arrivée constante de 20 requêtes/s pendant 30 secondes ; 10 utilisateurs virtuels préalloués, maximum 50 |
| Préchauffe | Catalogue créé avant l’essai ; aucune phase de préchauffe mesurée séparément |

| Résultat | Mesure |
| --- | --- |
| Requêtes et contrôles terminés | 601, tous réussis |
| Échecs HTTP et contrôles en échec | 0 et 0 |
| Débit | 20,0289 requêtes/s |
| Durée HTTP P50 | 5,9125 ms |
| Durée HTTP P95 | 7,136 ms |
| Durée HTTP P99 | 7,6254 ms |
| Durée HTTP moyenne et maximale | 5,3556 ms et 9,274 ms |
| Seuils | P95 < 500 ms, P99 < 1 000 ms et erreurs HTTP < 1 % : tous respectés |
| Interruptions | Aucune itération interrompue signalée |

Le résumé brut k6 est conservé dans `performance-results.json`. Dans ce format historique, la valeur `false` associée à un seuil signifie que le seuil n’a pas échoué. Tous les seuils étaient respectés dans la sortie de l’outil. Ce débit reste volontairement inférieur au limiteur applicatif et ne mesure pas la saturation. TLS, la latence réseau, les ressources des conteneurs, les historiques volumineux, la contention des écritures, les répliques et le stockage peuvent modifier fortement les résultats. Répétez la procédure de `performance.md` sur l’infrastructure cible avant de fixer un SLO.

Cette mesure de référence est associée à l’empreinte du binaire figurant dans `tested-binary.sha256`. Aucune révision Git n’était disponible lors de l’essai. L’archive contient le code et les instructions de construction, sans les outils téléchargés ni les bases locales.
