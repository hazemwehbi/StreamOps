# StreamOps

StreamOps est une application backend en Go dédiée à la gestion des services de streaming, de leurs environnements, de leur état opérationnel et des incidents associés. Son architecture de monolithe modulaire repose sur PostgreSQL et une API REST. Le dépôt comprend le code, les migrations SQL, les tests, les conteneurs, l’intégration continue et la documentation de déploiement. La diffusion des flux vidéo relève des services gérés, et non de cette application.

## Démarrage rapide

Installez Docker avec Compose, démarrez son moteur, puis ouvrez un terminal dans le dossier du projet.

Windows PowerShell :

```powershell
pwsh -File scripts/bootstrap.ps1
docker compose up --build -d
docker compose ps
curl.exe http://localhost:8080/readyz
```

Linux et macOS :

```sh
sh scripts/bootstrap.sh
docker compose up --build -d
docker compose ps
curl http://localhost:8080/readyz
```

Le script génère des mots de passe et des jetons aléatoires dans `.env` et `.secrets`. Il refuse de remplacer une configuration existante. Conservez ces fichiers confidentiels. Compose attend la disponibilité de PostgreSQL, applique les migrations avec un compte propriétaire distinct, puis démarre l’API avec un rôle de base de données limité. L’adresse locale par défaut est `http://localhost:8080`.

## Utilisation de l’API

Exemple PowerShell permettant de charger les paramètres sans afficher les jetons :

```powershell
$settings = @{}
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { $settings[$matches[1]] = $matches[2] }
}
$headers = @{ Authorization = "Bearer $($settings['ADMIN_TOKEN'])" }
$service = Invoke-RestMethod -Method Post -Uri http://localhost:8080/v1/services `
  -Headers $headers -ContentType application/json `
  -Body '{"service_key":"video-api","name":"API vidéo","environment":"production"}'
Invoke-RestMethod "http://localhost:8080/v1/services/$($service.id)" -Headers $headers
$incident = Invoke-RestMethod -Method Post `
  -Uri "http://localhost:8080/v1/services/$($service.id)/incidents" `
  -Headers $headers -ContentType application/json `
  -Body '{"title":"Lecture indisponible","severity":"critical"}'
Invoke-RestMethod -Method Patch -Uri "http://localhost:8080/v1/incidents/$($incident.id)" `
  -Headers $headers -ContentType application/json `
  -Body '{"status":"resolved","resolution_summary":"Capacité rétablie"}'
```

Consultez le [guide de l’API](docs/api.md) pour les exemples curl et les réponses, ainsi que le [contrat OpenAPI](docs/openapi.json) pour la description complète.

## Fonctionnalités

- Création, consultation et modification des services et de leur état, avec clé unique, filtres et pagination.
- Création et classification des incidents, confirmation, résolution, horodatages et historique transactionnel.
- Enregistrement et consultation des mesures de santé fournies par les clients, sans ordonnanceur de sondes externes.
- Jetons distincts pour l’administration, la lecture et les métriques ; validation JSON, limites de taille, CORS explicite et limitation de débit.
- Journaux JSON, identifiant par requête, métriques Prometheus et traçage HTTP facultatif avec OpenTelemetry.
- Migrations SQL embarquées, sommes de contrôle, verrou de migration et arrêt progressif.
- Image Docker à plusieurs étapes, exécution sans privilèges, PostgreSQL interne et GitHub Actions pour les tests, la construction et la recherche de vulnérabilités.

## Développement et tests

L’exécution directe nécessite Go 1.26 ou ultérieur et PostgreSQL 17 ou ultérieur. Utilisez une version corrective à jour ; la validation locale utilise Go 1.27.3. Exportez les variables décrites dans `.env.example` dans l’environnement du processus. Seul Compose charge automatiquement `.env`. Exécutez `go run ./cmd/api migrate`, puis `go run ./cmd/api`.

```sh
go test ./...
go vet ./...
go test -race ./...
go build -o bin/streamops ./cmd/api
```

Pour les tests d’intégration, définissez `TEST_DATABASE_URL` vers une base réservée aux tests, puis lancez :

```sh
go test -count=1 -v ./tests/integration
```

Chaque test crée un schéma aléatoire isolé et le supprime ensuite. Le compte utilisé doit pouvoir créer des schémas. Sans cette variable, les tests PostgreSQL sont explicitement ignorés. L’intégration continue fournit une base réelle et active le détecteur de courses de données. Son utilisation locale nécessite également un compilateur C compatible. Les résultats figurent dans le [rapport de vérification](docs/verification.md).

Le [guide d’exploitation](docs/operations.md) décrit la supervision, les sauvegardes et le déploiement en production. Pour créer des données synthétiques dans un environnement de développement, définissez `ALLOW_DEV_SEED=true` et `ADMIN_TOKEN`, puis lancez `go run ./cmd/seed`. Le [guide des performances](docs/performance.md) décrit les essais de charge.

## Architecture et livraison

Consultez l’[architecture](docs/architecture.md), les [décisions techniques](docs/decisions/0001-modular-monolith.md) et la [couverture des exigences](docs/requirements.md). Les procédures de déploiement sont fournies ; aucune infrastructure cloud n’a été provisionnée. Le domaine, TLS et les sauvegardes de production se configurent chez l’hébergeur. L’authentification repose sur des jetons de rôles partagés ; une intégration OIDC et la gestion de plusieurs locataires nécessitent des exigences complémentaires.

Les mesures locales ne constituent pas une estimation de capacité en production. Le rapport de vérification précise les contrôles réalisés et leurs limites. Licence MIT.

Pour produire une archive propre excluant les secrets, les outils locaux et les bases de test :

```powershell
pwsh -File scripts/package.ps1
```

Le fichier obtenu est `dist/StreamOps-FR.zip`.
