# Procédure reproductible de mesure des performances

Toute affirmation de débit ou de latence doit correspondre à une mesure enregistrée. Les seuils du scénario k6 sont des objectifs d’acceptation, pas des résultats.

Une mesure de référence locale figure dans `performance-results.md`, avec son résumé JSON brut. Elle vérifie le fonctionnement du scénario fourni sans établir une capacité de production.

Installez k6, démarrez StreamOps et créez un catalogue représentatif avec l’API ou `go run ./cmd/seed`, réservé au développement. Définissez `BASE_URL` et `READ_TOKEN`, puis lancez `k6 run tests/load/scenario.js`. La variable facultative `RPS` vaut 20 par défaut et `DURATION` vaut 30s. Commencez sous la limite de débit du processus ; les réponses 429 sont comptées comme des échecs. Augmentez progressivement la charge et relevez séparément le point de saturation.

Enregistrez la révision, la date, le système, le processeur, la mémoire, les versions Go/PostgreSQL, les limites Docker, les volumes de données, les tailles de pool, les répliques, la configuration du débit, la topologie réseau, le nombre de données initiales, la préchauffe et la durée. Conservez la sortie k6 avec P50/P95/P99, débit, taux d’erreur et itérations abandonnées. Répétez après préchauffe. Observez la charge et les verrous PostgreSQL ; une liste vide sur un poste local ne prouve pas une capacité de production.

Exemples de plans à exécuter avec des identifiants réels sur des données hors production :

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, service_key, name, environment, status, created_at, updated_at
FROM services WHERE environment='production' AND status='degraded'
ORDER BY created_at DESC,id LIMIT 20;

EXPLAIN (ANALYZE, BUFFERS)
SELECT id, title, severity, status, opened_at
FROM incidents WHERE service_id='00000000-0000-0000-0000-000000000001'
ORDER BY opened_at DESC,id LIMIT 20;

SELECT state, wait_event_type, wait_event, count(*)
FROM pg_stat_activity WHERE datname=current_database()
GROUP BY state,wait_event_type,wait_event;
```

Exemples Prometheus :

```promql
sum(rate(streamops_http_requests_total[5m]))
histogram_quantile(0.95, sum by (le) (rate(streamops_http_duration_seconds_bucket[5m])))
histogram_quantile(0.99, sum by (le) (rate(streamops_http_duration_seconds_bucket[5m])))
```

Comparez le coût des totaux, des décalages importants, des lectures de pages et de l’attente des connexions avant d’ajouter un index ou un cache. Les requêtes avec filtres facultatifs peuvent utiliser un plan générique à grande échelle. Mesurez les requêtes préparées avec une charge réaliste et introduisez des variantes ciblées lorsque les résultats le justifient.
