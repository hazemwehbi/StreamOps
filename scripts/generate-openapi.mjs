// Contrat API reproductible. Commande : node scripts/generate-openapi.mjs
import { writeFileSync } from 'node:fs';
const ref = name => ({ $ref: `#/components/schemas/${name}` });
const string = (extra = {}) => ({ type: 'string', ...extra });
const enumeration = values => string({ enum: values.split(' ') });
const id = string({ format: 'uuid' });
const timestamp = string({ format: 'date-time' });
const environments = enumeration('development staging production');
const serviceStatus = enumeration('operational degraded outage maintenance');
const incidentStatus = enumeration('open acknowledged resolved');
const severity = enumeration('low medium high critical');
const name = string({ minLength: 1, maxLength: 200 });
const object = (properties, required = [], extra = {}) => ({ type: 'object', additionalProperties: false, properties, ...(required.length ? { required } : {}), ...extra });
const serviceInput = { service_key: string({ pattern: '^[a-z0-9][a-z0-9_-]{2,63}$' }), name, environment: environments, status: serviceStatus };
const incidentInput = { title: name, description: string({ maxLength: 10000 }), severity };
const healthInput = { status: enumeration('healthy unhealthy unknown'), latency_ms: { type: 'integer', minimum: 0, maximum: 86400000 }, details: string({ maxLength: 2000 }) };
const schemas = {
  ServiceCreate: object(serviceInput, ['service_key', 'name', 'environment']),
  ServicePatch: object({ name, environment: environments, status: serviceStatus }, [], { minProperties: 1 }),
  Service: object({ id, ...serviceInput, created_at: timestamp, updated_at: timestamp }, ['id','service_key','name','environment','status','created_at','updated_at']),
  IncidentCreate: object(incidentInput, ['title','severity']),
  IncidentPatch: object({ ...incidentInput, status: incidentStatus, resolution_summary: string({ maxLength: 4000 }) }, [], { minProperties: 1 }),
  Incident: object({ id, service_id: id, ...incidentInput, status: incidentStatus, opened_at: timestamp, acknowledged_at: { ...timestamp, nullable: true }, resolved_at: { ...timestamp, nullable: true }, resolution_summary: string() }, ['id','service_id','title','description','severity','status','opened_at','acknowledged_at','resolved_at','resolution_summary']),
  HealthCreate: object(healthInput, ['status']),
  HealthCheck: object({ id, service_id: id, ...healthInput, checked_at: timestamp }, ['id','service_id','status','latency_ms','details','checked_at']),
  Event: object({ id: { type: 'integer', format: 'int64' }, incident_id: id, from_status: string(), to_status: incidentStatus, summary: string(), occurred_at: timestamp }, ['id','incident_id','from_status','to_status','summary','occurred_at']),
  Error: object({ error: object({ code: string(), message: string(), request_id: string({ format: 'uuid' }) }, ['code','message','request_id']) }, ['error']),
  Health: object({ status: string() }, ['status']),
};
for (const item of ['Service','Incident','HealthCheck']) schemas[`${item}Page`] = object({ items: { type: 'array', items: ref(item) }, total: { type: 'integer' }, limit: { type: 'integer' }, offset: { type: 'integer' } }, ['items','total','limit','offset']);
const param = (name, schema) => ({ name, in: 'query', schema });
const pagination = [param('limit', { type: 'integer', minimum: 1, maximum: 100, default: 20 }), param('offset', { type: 'integer', minimum: 0, maximum: 1000000, default: 0 })];
const pathID = { name: 'id', in: 'path', required: true, schema: id };
const response = (description, schema) => ({ description, content: { 'application/json': { schema } } });
const operation = (summary, output, { input, status = 200, parameters = [], publicAccess = false, description = '' } = {}) => ({
  summary, description, ...(publicAccess ? { security: [] } : {}), parameters,
  ...(input ? { requestBody: { required: true, content: { 'application/json': { schema: ref(input) } } } } : {}),
  responses: { [status]: { ...response('Réussite', typeof output === 'string' ? ref(output) : output), headers: { 'X-Request-ID': { schema: id } } }, ...Object.fromEntries([400,401,403,404,405,409,413,415,429,500,503,504].map(code => [code,response('Erreur',ref('Error'))])) },
});
const paths = {
  '/healthz': { get: operation('État du processus','Health',{ publicAccess: true }) },
  '/readyz': { get: operation('Disponibilité de la base de données','Health',{ publicAccess: true }) },
  '/v1/services': {
    post: operation('Créer un service','Service',{ input: 'ServiceCreate',status: 201,description:'Le jeton d’administration est requis. service_key est unique dans toute l’application et immuable.' }),
    get: operation('Lister les services','ServicePage',{ parameters: [...pagination,param('environment',environments),param('status',serviceStatus)] }),
  },
  '/v1/services/{id}': {
    get: operation('Consulter un service','Service',{ parameters: [pathID] }),
    patch: operation('Modifier un service','Service',{ input:'ServicePatch',parameters:[pathID],description:'Le jeton d’administration est requis. Les valeurs null et les modifications vides sont rejetées.' }),
  },
  '/v1/services/{id}/incidents': { post: operation('Ouvrir un incident','Incident',{ input:'IncidentCreate',status:201,parameters:[pathID],description:'Le jeton d’administration est requis. Les nouveaux incidents ont toujours l’état open.' }) },
  '/v1/incidents': { get: operation('Lister les incidents','IncidentPage',{ parameters:[...pagination,param('service_id',id),param('status',incidentStatus),param('severity',severity),param('from',timestamp),param('to',timestamp)],description:'from et to sont les bornes inclusives RFC3339 de opened_at.' }) },
  '/v1/incidents/{id}': {
    get: operation('Consulter un incident','Incident',{ parameters:[pathID] }),
    patch: operation('Modifier un incident','Incident',{ input:'IncidentPatch',parameters:[pathID],description:'Le jeton d’administration est requis. Transitions : open -> acknowledged -> resolved ou open -> resolved. La résolution exige un resolution_summary non vide. Les incidents résolus sont immuables. Chaque modification réussie ajoute un événement.' }),
  },
  '/v1/incidents/{id}/history': { get: operation('Historique de l’incident',{ type:'array',items:ref('Event') },{ parameters:[pathID] }) },
  '/v1/services/{id}/health-checks': {
    post: operation('Enregistrer un contrôle de santé mesuré par le client','HealthCheck',{ input:'HealthCreate',status:201,parameters:[pathID],description:'Le jeton d’administration est requis. Le serveur horodate le contrôle ; cette route ne sonde pas d’URL externe.' }),
    get: operation('Lister les contrôles de santé','HealthCheckPage',{ parameters:[pathID,...pagination] }),
  },
  '/metrics': { get: { summary:'Exposition des métriques Prometheus',security:[{ MetricsBearer:[] }],responses:{ 200:{ description:'Format texte Prometheus',content:{ 'text/plain':{ schema:string() } } },401:response('Non autorisé',ref('Error')) } } },
};
const spec = { openapi:'3.0.3',info:{ title:'API REST StreamOps',version:'1.0.0',description:'Toutes les routes /v1 exigent un jeton Bearer. Jeton de lecture : GET/HEAD. Jeton d’administration : toutes les opérations. Corps JSON limité à 32768 octets. Les limites de chaînes sont exprimées en octets UTF-8. Les champs inconnus ou null et les paramètres de requête inconnus sont rejetés.' },servers:[{ url:'http://localhost:8080' }],security:[{ APIBearer:[] }],paths,components:{ securitySchemes:{ APIBearer:{ type:'http',scheme:'bearer',description:'Utilisez ADMIN_TOKEN ou READ_TOKEN selon l’opération.' },MetricsBearer:{ type:'http',scheme:'bearer',description:'Utilisez METRICS_TOKEN.' } },schemas } };
writeFileSync('docs/openapi.json', JSON.stringify(spec,null,2)+'\n');
console.log('Fichier docs/openapi.json généré');
