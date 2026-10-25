# Déploiement et exploitation

## Exécution locale avec Compose

Générez les identifiants une seule fois avec `pwsh -File scripts/bootstrap.ps1` sous Windows ou `sh scripts/bootstrap.sh` sous Linux et macOS. Démarrez Docker Desktop ou Docker Engine, puis lancez `docker compose up --build -d`. La commande `docker compose ps` affiche la santé de la base, la réussite de la tâche de migration et la disponibilité de l’API. Utilisez `docker compose logs -f api` et `curl http://localhost:8080/readyz` pour le diagnostic. `docker compose down` conserve les données. **`docker compose down -v` supprime le volume de la base.**

Le script d’initialisation crée le rôle d’exécution uniquement lors de la création d’un volume PostgreSQL vierge. Modifier ensuite les variables de mot de passe ne change pas les mots de passe existants. Effectuez leur rotation avec `ALTER ROLE ... PASSWORD` dans une session administrative sécurisée, mettez à jour les secrets et redémarrez les conteneurs concernés. Conservez `.env`, `.secrets` et les sauvegardes confidentiels. Le script de préparation ne remplace jamais les identifiants existants. Régénérez `.secrets/metrics_token` lors de la rotation de `METRICS_TOKEN`.

Pour la supervision, définissez `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://otel-collector:4318/v1/traces` dans `.env`, puis exécutez `docker compose --profile monitoring up --build -d`. Prometheus est disponible sur `http://localhost:9090` ; son jeton est monté sous forme de fichier secret. Le collecteur écrit les traces échantillonnées dans ses journaux. Remplacez l’exporteur de diagnostic par votre solution de stockage des traces pour les conserver et les rechercher. Le démarrage de l’application ne dépend pas du collecteur.

## Exécution directe

Prérequis : Go 1.26 ou ultérieur, PostgreSQL 17 ou ultérieur et configuration exportée dans l’environnement du processus. Utilisez les versions correctives à jour. Compose est le seul composant qui charge `.env` automatiquement. Variables obligatoires : `DATABASE_URL`, `ADMIN_TOKEN`, `READ_TOKEN` et `METRICS_TOKEN`. Valeurs par défaut : `HTTP_ADDR=:8080`, `DB_POOL_MAX=10`, `DB_TIMEOUT=5s`, `RATE_LIMIT_RPS=100`, `RATE_LIMIT_BURST=200` ; `CORS_ORIGIN` reste vide. L’arrêt progressif dispose de 15 secondes. La variable facultative `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` contient une URL complète se terminant par `/v1/traces`. Les jetons doivent être robustes et distincts. Encodez les caractères réservés des mots de passe dans l’URI `DATABASE_URL` ; les mots de passe générés par les scripts sont compatibles avec ce format.

Lancez `go run ./cmd/api migrate` avec le compte propriétaire avant `go run ./cmd/api` avec le rôle d’exécution. En production, utilisez TLS avec `sslmode=verify-full` et la chaîne de certificats attendue. Compose désactive TLS sur son réseau interne de développement. Pour un rôle géré manuellement, accordez `USAGE` sur le schéma, `SELECT/INSERT/UPDATE` sur les tables et `USAGE/SELECT` sur les séquences, puis retirez `INSERT/UPDATE` sur `schema_migrations` et `UPDATE` sur `incident_events`. Le rôle d’exécution doit conserver `SELECT` sur `schema_migrations` pour le contrôle au démarrage.

## Mise en production

1. Exécutez l’intégration continue avec la version majeure PostgreSQL cible. Construisez l’image depuis la révision exacte validée et fixez les images par leur empreinte de registre dans le manifeste de livraison. Analysez les images et les dépendances Go et traitez les vulnérabilités.
2. Provisionnez PostgreSQL, un compte propriétaire pour les migrations et un rôle d’exécution distinct. Stockez les identifiants dans le gestionnaire de secrets. Limitez l’accès réseau à la base à l’API et aux migrations. La somme des pools de toutes les répliques doit rester inférieure à la capacité de connexion de PostgreSQL.
3. Sauvegardez la base et vérifiez un exercice récent de restauration. Exécutez une tâche `streamops migrate` avec la nouvelle image. Vérifiez le schéma avant d’envoyer le trafic. Privilégiez les migrations compatibles avec les versions successives et coordonnez les changements incompatibles.
4. Déployez avec l’UID non privilégié 65532, un système de fichiers racine en lecture seule, les capacités Linux retirées, l’élévation de privilèges interdite, des limites de ressources et au moins 25 secondes de délai de terminaison. Configurez `/healthz` pour la vitalité et `/readyz` pour la disponibilité sur le port 8080. La commande interne `probe` suppose ce port ; utilisez une sonde HTTP externe si vous le changez.
5. Placez l’API derrière une entrée TLS. Restreignez l’accès aux métriques et aux routes de santé selon votre réseau. Définissez une origine CORS exacte uniquement si un client navigateur l’exige. Appliquez les limites de connexions et de requêtes en périphérie ; le limiteur applicatif s’applique aux requêtes authentifiées `/v1` par réplique.
6. Vérifiez la création, la consultation et la modification des services, la résolution des incidents, l’historique et les mesures de santé. Confirmez la collecte Prometheus, le taux d’erreur et le comportement du pool. Ne rétablissez une ancienne image que si le schéma reste compatible. Toute restauration de données suit la procédure de reprise approuvée.

Les accès d’hébergement, DNS et TLS ne sont pas inclus. Aucun environnement cloud n’a été provisionné ou déployé.

## Sauvegarde et restauration

Utilisez `pg_dump --format=custom` avec le compte propriétaire. Chiffrez les sauvegardes et appliquez une rétention hors de l’hôte applicatif. Restaurez dans une base isolée vide avec `pg_restore --no-owner --no-acl --dbname=...`. Recréez les permissions du rôle d’exécution, appliquez les migrations et vérifiez les volumes de données ainsi que les parcours API avant de basculer le trafic. Les sauvegardes peuvent contenir des données confidentielles et ne doivent jamais être versionnées. La haute disponibilité, l’automatisation des sauvegardes et les objectifs RPO/RTO relèvent du plan d’hébergement et doivent y être testés.

## Diagnostic

401 : vérifiez le rôle et le jeton sans les journaliser. 403 : vérifiez les permissions et l’origine CORS exacte. 409 : contrôlez l’unicité de `service_key` ou l’immuabilité d’un incident résolu. 429 : examinez le débit et les limites par instance ; ne répétez pas aveuglément les écritures. 503 sur `/readyz` : vérifiez la base, les identifiants et la saturation du pool. 504 : examinez les verrous et les plans SQL avant d’augmenter `DB_TIMEOUT`. En cas de schéma invalide au démarrage, appliquez les migrations avec le propriétaire sans modifier les lignes de suivi ni les fichiers appliqués. Pour une migration Compose en échec, consultez `docker compose logs migrate`, corrigez la cause et relancez la tâche.

Le signal `SIGTERM` déclenche l’arrêt progressif. Les requêtes en cours disposent de 15 secondes, puis les connexions restantes sont fermées. Le pool et les ressources de traçage sont ensuite libérés. L’arrêt de l’exporteur dispose d’un délai distinct de cinq secondes.
