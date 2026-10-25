import http from 'k6/http';
import { check } from 'k6';
export const options = {
  scenarios: { services: { executor: 'constant-arrival-rate', rate: Number(__ENV.RPS || 20), timeUnit: '1s', duration: __ENV.DURATION || '30s', preAllocatedVUs: 10, maxVUs: 50 } },
  thresholds: { http_req_failed: ['rate<0.01'], http_req_duration: ['p(95)<500','p(99)<1000'] },
  summaryTrendStats: ['avg','min','med','p(50)','p(95)','p(99)','max'],
};
export function setup() {
  if (!__ENV.READ_TOKEN) throw new Error('READ_TOKEN est obligatoire');
}
export default function () {
  const response = http.get(`${__ENV.BASE_URL || 'http://localhost:8080'}/v1/services?limit=20`, { headers: { Authorization: `Bearer ${__ENV.READ_TOKEN}` } });
  check(response, { 'liste accessible': r => r.status === 200 });
}
