import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },   
    { duration: '2m',  target: 20 },   
    { duration: '30s', target: 0 },    
  ],
  thresholds: {
  http_req_duration: ['p(95)<2000'],
  http_req_failed:   ['rate<0.01'],
  },
};

const BASE = 'http://localhost:8080';

export default function () {
  const payload = JSON.stringify({ item: 'apple' });
  const headers = { 'Content-Type': 'application/json' };

  const postRes = http.post(`${BASE}/order`, payload, { headers });
  check(postRes, { 'order created': (r) => r.status === 200 });

  const getRes = http.get(`${BASE}/orders`);
  check(getRes, { 'orders listed': (r) => r.status === 200 });

  sleep(0.5);
}