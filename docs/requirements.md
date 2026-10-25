# Couverture des exigences

Référence : `Description_Generale_StreamOps_FR.docx`. Le périmètre définit un backend et prévoit le stockage des contrôles de santé ainsi que la télémétrie comme extensions possibles. Ces extensions sont incluses.

| Domaine | Réalisation | Vérification |
| --- | --- | --- |
| Catalogue, unicité et états des services | Validation métier, table services, création, consultation, liste et modification REST | Tests unitaires, HTTP et PostgreSQL |
| Incidents et cycle de vie | Verrous de ligne, horodatages, résumé obligatoire, historique transactionnel | Tests des transitions, de concurrence et de l’API |
| Vitalité et disponibilité | `/healthz`, `/readyz` et délai de vérification PostgreSQL | Tests HTTP et contrôle du binaire |
| Contrôles de santé par service | POST/GET, contraintes et stockage paginé | Validation et tests PostgreSQL/API |
| Architecture Go modulaire | cmd, config, domain, service, repository, http et observability | Construction, vet, documentation et ADR |
| Persistance PostgreSQL | Requêtes pgx paramétrées, pool, contraintes, clés étrangères et index | Tests réels sous PostgreSQL 17.11 |
| Migrations versionnées | SQL embarqué, sommes de contrôle, verrou consultatif et atomicité | Réapplication, détection d’altération et annulation transactionnelle |
| Sécurité | Jetons par rôle, compte SQL limité, limites de taille, JSON strict, CORS et débit | Tests HTTP et permissions SQL |
| Observabilité | Journaux JSON, identifiants, dimensions Prometheus bornées, traces HTTP OTLP facultatives | Tests des métriques et de l’export de traces |
| Arrêt et délais | Signaux, arrêt progressif, échéances HTTP/SQL, fermeture du pool et de l’exporteur | Annulation SQL et drainage des requêtes |
| Déploiement Docker | Image statique non privilégiée, Compose, tâche de migration et supervision facultative | Configuration vérifiée ; démarrage des conteneurs prévu dans la CI |
| Qualité et intégration continue | Tests Go, intégration, race, vet, formatage, construction et govulncheck | Résultats locaux dans verification.md ; exécution GitHub après publication du dépôt |
| Performances | Données synthétiques, scénario k6, percentiles et procédure EXPLAIN | Mesure locale avec 1 000 services et 601 requêtes réussies ; aucune capacité de production présumée |
| Documentation et licence | Guide de démarrage, exemples et OpenAPI, architecture, ADR, exploitation et licence MIT | Contrôle des fichiers et validation OpenAPI |

Limites du périmètre : aucun moteur de diffusion vidéo, interface graphique, ordonnanceur de sondes externes, émetteur OIDC, cloisonnement multilocataire, provisionnement cloud ou système de haute disponibilité n’est spécifié. L’exécution Docker nécessite un moteur Docker. Les jetons partagés conviennent au déploiement privé fourni sans fournir d’attribution nominative. L’historique renvoie actuellement tous les événements d’un incident ; les listes de services, d’incidents et de contrôles sont paginées. Des historiques très volumineux nécessiteraient une pagination et une politique de conservation complémentaires.
