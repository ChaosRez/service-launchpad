import http from "k6/http";
import { check } from "k6";

const baseUrl =
  __ENV.BASE_URL ||
  "http://fastapi-service.service-launchpad-dev.svc.cluster.local:8000";
const runtimeProfile = __ENV.RUNTIME_PROFILE || "long";
const rate = Number(__ENV.RATE || 20);
const duration = __ENV.DURATION || "5m";
const preAllocatedVUs = Number(__ENV.PRE_ALLOCATED_VUS || 20);
const maxVUs = Number(__ENV.MAX_VUS || 200);
const testId = __ENV.TESTID || "manual";

export const options = {
  discardResponseBodies: true,
  scenarios: {
    chat_completions: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs,
      maxVUs,
      tags: {
        scenario: "chat_completions",
        testid: testId,
      },
    },
  },
  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
  },
};

const requestBody = JSON.stringify({
  runtime_profile: runtimeProfile,
});

const params = {
  headers: {
    "Content-Type": "application/json",
  },
  tags: {
    name: "chat_completions",
    runtime_profile: runtimeProfile,
    service: "fastapi-service",
    testid: testId,
  },
  timeout: "30s",
};

export default function () {
  const response = http.post(
    `${baseUrl}/v1/chat/completions`,
    requestBody,
    params,
  );

  check(response, {
    "status is 200": (r) => r.status === 200,
    "chat completion payload": (r) =>
      r.headers["Content-Type"] &&
      r.headers["Content-Type"].includes("application/json"),
  });
}
