# ADR 0001 Monolithe modulaire et SQL explicite

Statut : accepté.

Un déploiement et une base relationnelle suffisent au périmètre défini, sans cohérence distribuée ni découverte de services. `net/http` fournit le routage, les limites, les délais et l’arrêt progressif. pgx apporte le pool PostgreSQL et les transactions annulables. Les contraintes SQL complètent la validation de l’API. Les consultations passent par une interface de persistance ; les validations et les transitions sont indépendantes de la base. Les verrous de ligne protègent les mises à jour sans exposer de numéro de version optimiste aux clients.

Conséquences : les modules partagent un cycle de livraison ; le calcul des totaux ajoute du travail à PostgreSQL ; la pagination par décalage convient aux catalogues de taille modérée. Le découpage en services, le cache ou la pagination par curseur seront motivés par des mesures. Compose sépare le compte propriétaire des migrations du rôle d’exécution. Le moteur de migration applique uniquement des évolutions successives.
