import http from "k6/http";
import { check } from "k6";
import exec from "k6/execution";

const profile = __ENV.LOAD_PROFILE || "smoke";
const profiles = {
  smoke: { rate: 5, duration: "30s", preAllocatedVUs: 10, maxVUs: 50 },
  steady: { rate: 500, duration: __ENV.LOAD_DURATION || "24h", preAllocatedVUs: 500, maxVUs: 5000 },
  peak: { rate: 2000, duration: __ENV.LOAD_DURATION || "1h", preAllocatedVUs: 1500, maxVUs: 10000 },
};

if (!profiles[profile]) {
  throw new Error(`unsupported LOAD_PROFILE ${profile}`);
}

const selected = profiles[profile];

export const options = {
  discardResponseBodies: false,
  scenarios: {
    payments: {
      executor: "constant-arrival-rate",
      rate: selected.rate,
      timeUnit: "1s",
      duration: selected.duration,
      preAllocatedVUs: selected.preAllocatedVUs,
      maxVUs: selected.maxVUs,
      gracefulStop: "2m",
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<200"],
    http_req_failed: ["rate<0.001"],
    checks: ["rate>0.999"],
  },
};

const baseURL = (__ENV.BASE_URL || "http://localhost:8888").replace(/\/$/, "");
const token = __ENV.ACCESS_TOKEN || "";
const callbackEndpointID = __ENV.CALLBACK_ENDPOINT_ID || "load-test-endpoint";
const tenantPrefix = __ENV.IDEMPOTENCY_PREFIX || "k6";

export default function () {
  const unique = `${tenantPrefix}-${exec.scenario.iterationInTest}`;
  const body = JSON.stringify({
    money: { amount_minor: 10050, currency: "USD", exponent: 2 },
    customer_id: `customer-${exec.vu.idInTest}`,
    callback_endpoint_id: callbackEndpointID,
    metadata: { source: "k6", profile },
  });
  const response = http.post(`${baseURL}/api/v2/payments`, body, {
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      "Idempotency-Key": unique,
    },
    tags: { operation: "create-payment-v2" },
    timeout: "5s",
  });

  check(response, {
    "payment created or replayed": (result) => result.status === 200 || result.status === 201,
    "transaction id returned": (result) => {
      try {
        return Boolean(result.json("transaction_id"));
      } catch (_) {
        return false;
      }
    },
  });
}

