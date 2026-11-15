import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE || 'http://localhost:8080';
const RATE = Number(__ENV.RATE || 300);
const headers = { 'Content-Type': 'application/json' };

export const options = {
    scenarios: {
        site: {
            executor: 'constant-arrival-rate',
            rate: RATE,
            timeUnit: '1s',
            duration: __ENV.DURATION || '60s',
            preAllocatedVUs: 200,
            maxVUs: 1000,
        },
    },
    summaryTrendStats: ['avg', 'p(50)', 'p(95)', 'p(99)', 'max'],
};

const rnd = (n) => Math.floor(Math.random() * n) + 1;

export default function () {
    const roll = Math.random();
    let res;
    if (roll < 0.55) {
        res = http.get(`${BASE}/comments/${rnd(2000)}`, { tags: { op: 'read' } });
    } else if (roll < 0.80) {
        res = http.get(`${BASE}/posts/${rnd(50)}/comments`, { tags: { op: 'read' } });
    } else if (roll < 0.92) {
        res = http.post(`${BASE}/comments/${rnd(20)}/vote`, JSON.stringify({ delta: 1 }), { headers, tags: { op: 'write' } });
    } else if (roll < 0.97) {
        res = http.post(`${BASE}/comments`,
            JSON.stringify({ post_id: rnd(50), author: `user_${rnd(300)}`, body: `load test ${Date.now()}` }),
            { headers, tags: { op: 'write' } });
    } else {
        res = http.patch(`${BASE}/comments/${rnd(2000)}`, JSON.stringify({ body: `edited ${Date.now()}` }), { headers, tags: { op: 'write' } });
    }
    check(res, { 'status < 500': (r) => r.status < 500 });
}