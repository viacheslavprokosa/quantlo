import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
    vus: 50, // virtual users
    duration: '30s',
};

const BASE_URL = 'http://localhost:8080';

export default function () {
    const payload = JSON.stringify({
        account_id: 'user_test',
        resource_type: 'api_credits',
        amount: 1,
        idempotency_key: `key-${Math.random()}`,
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const res = http.post(`${BASE_URL}/spend`, payload, params);

    check(res, {
        'is status 200': (r) => r.status === 200,
    });

    sleep(0.1);
}
