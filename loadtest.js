import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '5s', target: 50 },  // Ramp up to 50 virtual users
    { duration: '10s', target: 50 }, // Sustain load
    { duration: '5s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<10', 'p(99)<20'], // p99 must be under 20ms
    http_req_failed: ['rate<0.01'],              // Less than 1% failure
  },
};

export default function () {
  const res = http.get('http://localhost:8080/predict');
  check(res, {
    'is status 200': (r) => r.status === 200,
  });
  sleep(0.1);
}
