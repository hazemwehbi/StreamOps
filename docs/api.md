# Guide de l’API REST

Le contrat exploitable par les outils est disponible dans `docs/openapi.json`, au format OpenAPI 3.0.3. Importez-le dans votre client API ou Swagger UI. Les dates suivent RFC3339 en UTC et les identifiants sont des UUID. Les limites de chaînes JSON sont exprimées en octets UTF-8. Les champs gérés par le serveur, tels que les identifiants et les dates, ne sont pas acceptés à la création. Les champs inconnus et les valeurs null sont rejetés.

Définissez `ADMIN_TOKEN` et `READ_TOKEN` dans l’environnement à partir de votre configuration confidentielle. Ne placez pas de vrais jetons dans le dépôt ni dans les exemples. Les commandes suivantes utilisent un interpréteur POSIX ; les exemples PowerShell figurent dans le README.

```sh
# Création : code 201 et en-tête Location
curl -i http://localhost:8080/v1/services \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"service_key":"video-api","name":"API vidéo","environment":"production"}'

# Liste : code 200 ; état operational par défaut à la création
curl 'http://localhost:8080/v1/services?environment=production&limit=20&offset=0' \
  -H "Authorization: Bearer $READ_TOKEN"

# Remplacez SERVICE_ID et INCIDENT_ID par les UUID retournés
curl -X PATCH http://localhost:8080/v1/services/SERVICE_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"degraded"}'

curl http://localhost:8080/v1/services/SERVICE_ID/incidents \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"Erreurs de lecture","description":"Taux d’erreur élevé","severity":"critical"}'

curl -X PATCH http://localhost:8080/v1/incidents/INCIDENT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"acknowledged"}'

curl -X PATCH http://localhost:8080/v1/incidents/INCIDENT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"resolved","resolution_summary":"Capacité des encodeurs rétablie"}'

curl 'http://localhost:8080/v1/incidents?status=resolved&severity=critical&from=2026-01-01T00:00:00Z' \
  -H "Authorization: Bearer $READ_TOKEN"

curl http://localhost:8080/v1/incidents/INCIDENT_ID/history \
  -H "Authorization: Bearer $READ_TOKEN"

curl http://localhost:8080/v1/services/SERVICE_ID/health-checks \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"healthy","latency_ms":24,"details":"HTTP 200"}'
```

Exemple de réponse à la création d’un service, avec identifiants et dates illustratifs :

```json
{"id":"d644f6eb-270e-4817-a145-2bf880a8845f","service_key":"video-api","name":"API vidéo","environment":"production","status":"operational","created_at":"2026-10-25T10:00:00Z","updated_at":"2026-10-25T10:00:00Z"}
```

Réponse d’une liste :

```json
{"items":[],"limit":20,"offset":0,"total":0}
```

Réponse d’erreur :

```json
{"error":{"code":"validation_error","message":"données ou transition d’incident invalides","request_id":"d644f6eb-270e-4817-a145-2bf880a8845f"}}
```

Chaque réponse comprend `X-Request-ID`. Les erreurs utilisent les codes 400 pour les données invalides, 401 pour des identifiants absents ou invalides, 403 pour des droits insuffisants ou une origine interdite, 404 pour une ressource absente, 405 pour une méthode non prise en charge, 409 pour un conflit, 413 pour un corps dépassant 32768 octets, 415 pour un type de contenu non pris en charge, 429 pour le dépassement du débit, 500 pour une erreur interne, 503 pour une dépendance indisponible et 504 pour une échéance dépassée. L’authentification est vérifiée avant les ressources. Les routes de santé sont accessibles sans jeton dans le réseau configuré. Les métriques exigent `METRICS_TOKEN`.

Les niveaux de gravité sont `low`, `medium`, `high` et `critical`. Les transitions autorisées sont `open -> acknowledged -> resolved` et `open -> resolved`. Un résumé est obligatoire pour résoudre un incident. Un incident résolu ne peut être ni modifié ni rouvert. Les états de contrôle de santé sont `healthy`, `unhealthy` et `unknown`. Un contrôle enregistré ne modifie pas automatiquement l’état du service.
