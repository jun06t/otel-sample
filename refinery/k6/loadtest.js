import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 10 },
    { duration: '10s', target: 0 },
  ],
};

export default function () {
  const res = http.get('http://gateway:8000/hello');
  check(res, {
    'status is 200 or 500': (r) => r.status === 200 || r.status === 500,
  });
  sleep(0.5);
}
