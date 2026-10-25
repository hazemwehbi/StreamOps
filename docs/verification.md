# Rapport de vérification

Date : 25 octobre 2026. Environnement : Windows x64, Go 1.27.3 et LLVM MinGW 20261020 avec CGO pour le détecteur de courses de données. Base réelle : PostgreSQL 17.11 sur une instance locale isolée, port 55432. Chaque test crée puis supprime un schéma unique.

| Contrôle | Résultat |
| --- | --- |
| `go test ./...` | Réussi, avec PostgreSQL réel et TEST_DATABASE_URL configuré |
| `go test -race -count=1 -coverpkg=./internal/...,./migrations -coverprofile=coverage.out ./...` | Réussi, aucune course de données détectée |
| Couverture des paquets internes et migrations | 84,6 % des instructions ; commandes exclues de ce périmètre |
| `go vet ./...` | Réussi |
| `gofmt` | Formatage appliqué et vérifié |
| Construction statique | Réussie avec CGO_ENABLED=0 |
| Contrôle fonctionnel du binaire | Migrations, disponibilité, création de service et d’incident, puis résolution directe réussies |
| Intégrité SQL | Unicité, clés étrangères, contraintes CHECK, annulation d’une transition invalide, filtres et pagination vérifiés |
| Concurrence | Huit créations simultanées d’une même clé : une réussite et sept conflits ; confirmations d’incident sérialisées |
| Migrations | Réapplication idempotente ; somme de contrôle altérée rejetée au démarrage et par le moteur de migration |
| Permissions SQL | Le rôle applicatif ne peut créer des tables, supprimer des services, modifier le suivi des migrations ni modifier l’historique ; la modification autorisée d’un service réussit |
| Contrat et sécurité HTTP | Rôles, limites de corps, type de contenu, champs null/inconnus, UUID, filtres, limitation de débit, erreurs, refus CORS et métriques de route vérifiés |
| Télémétrie | Export d’une trace échantillonnée vers un récepteur HTTP OTLP ; dimensions Prometheus bornées et récupération des paniques vérifiées |
| `govulncheck` v1.9.0 | Aucune vulnérabilité trouvée avec golang.org/x/net v0.60.0 |
| OpenAPI 3.0.3 | Validation réussie avec @apidevtools/swagger-parser |
| YAML et Compose | Configuration principale et profil de supervision validés par Docker Compose v5.6.1 ; fichiers YAML de déploiement et CI valides |
| Préparation | Génération PowerShell des identifiants vérifiée ; aucun secret local dans l’archive |
| Mesure de référence k6 | 1 000 services, 601 requêtes réussies en 30 secondes à environ 20 requêtes/s ; P95 7,136 ms et P99 7,6254 ms ; détails dans performance-results.md |

Le détecteur de courses utilise un compilateur C local ; le binaire applicatif est construit statiquement. Les dépendances sont fixées dans `go.mod` et `go.sum`. Le résultat de recherche de vulnérabilités dépend de la date du contrôle et doit être renouvelé lors de chaque livraison. La couverture ne garantit pas l’absence de défauts et n’inclut pas l’infrastructure de production.

Non exécutés sur ce poste : construction et démarrage des conteneurs, faute de moteur Docker ; script de préparation Linux ; exécution du workflow sur GitHub ; TLS et entrée de production ; déploiement cloud ; exercices de restauration et de charge en production. La CI prévoit les tests PostgreSQL avec race, la construction de l’image et un contrôle de démarrage Compose après publication sur GitHub. Les conteneurs Prometheus et collecteur ont été configurés et validés statiquement, sans démarrage local.

Le processus Windows du contrôle fonctionnel est arrêté explicitement après l’essai. Le drainage progressif est vérifié séparément en annulant le contexte du serveur pendant une requête active. La réception de SIGTERM dans un conteneur Linux reste un contrôle de déploiement.

Le scénario de charge de référence et la génération des données ont réussi. L’interrogation facultative des informations matérielles par CIM a ensuite été refusée ; ces informations ont été relevées séparément par le runtime .NET. Le processus et la base de mesure ont été nettoyés dans le bloc de finalisation.
