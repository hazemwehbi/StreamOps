# ADR 0002 Jetons de rôles et déploiement privé

Statut : accepté pour cette livraison.

Le périmètre exige des contrôles d’accès administratifs sans définir de fournisseur d’identité. StreamOps valide donc des jetons Bearer distincts pour l’administration, la lecture et les métriques, avec une comparaison en temps constant. Ils doivent être distincts et contenir au moins 32 octets ; les scripts génèrent des valeurs hexadécimales aléatoires de 256 bits. Les origines inconnues sont rejetées et CORS est désactivé par défaut. Un seau de jetons commun aux requêtes authentifiées borne le trafic par processus, sans registre de clients de taille illimitée. Il ne représente ni un quota individuel ni une limite commune à plusieurs répliques.

Compose lie les ports applicatifs et de supervision à l’interface locale, maintient PostgreSQL sur le réseau interne et utilise un conteneur applicatif statique non privilégié. Le rôle d’exécution ne peut ni effectuer de DDL ni modifier l’historique des incidents ; les migrations utilisent le propriétaire. En production, l’entrée réseau fournit TLS et une limitation de débit périphérique. Pour des utilisateurs nominatifs ou plusieurs locataires, remplacez les jetons partagés par une validation OIDC/JWT avec un émetteur approuvé et ajoutez le cloisonnement des données. Cette intégration devra être spécifiée séparément.
