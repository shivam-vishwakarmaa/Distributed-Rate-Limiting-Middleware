import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '10s', target: 100 },
        { duration: '30s', target: 5000 },
        { duration: '10s', target: 0 },
    ],
};

export default function () {
    const res = http.get('http://localhost:8080/');
    check(res, {
        'is status 200 or 429': (r) => r.status === 200 || r.status === 429,
    });
}
