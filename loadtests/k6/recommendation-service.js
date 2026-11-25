// Constant open-model load for recommendation-service.
//
// constant-arrival-rate starts RATE iterations per second regardless of how slow the service is,
// so latency degradation shows up as latency (and dropped_iterations), not as a silently lower
// request rate the way a closed VU loop would hide it. That is what you want when the question is
// "does the HPA keep p99 under the SLO at N rps".
//
// Traffic mix (tunable via env):
//   HOME_SHARE      share of home-feed requests; the rest are product-page requests
//   KNOWN_USER_SHARE share of requests for users that exist in the model output; the rest are
//                   unseen ids that exercise the cold-start fallback
//   ANON_SHARE      share of requests without any userId
import http from "k6/http";
import { check } from "k6";

const baseUrl =
  __ENV.BASE_URL ||
  "http://recommendation-service.service-launchpad-reco.svc.cluster.local:8080";
const rate = Number(__ENV.RATE || 200);
const duration = __ENV.DURATION || "24h";
const preAllocatedVUs = Number(__ENV.PRE_ALLOCATED_VUS || 50);
const maxVUs = Number(__ENV.MAX_VUS || 500);
const testId = __ENV.TESTID || "reco-constant";
const users = Number(__ENV.USERS || 20000);
const products = Number(__ENV.PRODUCTS || 5000);
const homeShare = Number(__ENV.HOME_SHARE || 0.7);
const knownUserShare = Number(__ENV.KNOWN_USER_SHARE || 0.85);
const anonShare = Number(__ENV.ANON_SHARE || 0.05);
const limit = Number(__ENV.LIMIT || 24);
const sloP99 = __ENV.SLO_P99 || "50";

export const options = {
  discardResponseBodies: true,
  // Keep the summary small; this runs for hours and the dashboard is the real report.
  summaryTrendStats: ["p(50)", "p(95)", "p(99)", "max"],
  scenarios: {
    recommendations: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs,
      maxVUs,
      tags: { testid: testId },
    },
  },
  thresholds: {
    // Reported, not enforced: a constant generator should keep running when the SLO breaks,
    // because the interesting part is watching the HPA recover from it.
    http_req_failed: ["rate<0.01"],
    "http_req_duration{endpoint:home}": [`p(99)<${sloP99}`],
    "http_req_duration{endpoint:product}": [`p(99)<${sloP99}`],
  },
};

function userParam() {
  const r = Math.random();
  if (r < anonShare) return "";
  if (r < anonShare + (1 - anonShare) * knownUserShare) {
    return `userId=u${Math.floor(Math.random() * users)}&`;
  }
  return `userId=new${Math.floor(Math.random() * 1e9)}&`;
}

export default function () {
  const isHome = Math.random() < homeShare;
  const endpoint = isHome ? "home" : "product";
  const path = isHome
    ? `/v1/recommendations/home?${userParam()}limit=${limit}`
    : `/v1/recommendations/product/${Math.floor(Math.random() * products)}?${userParam()}limit=${limit}`;

  const res = http.get(`${baseUrl}${path}`, {
    tags: { endpoint, service: "recommendation-service", testid: testId, name: endpoint },
    timeout: "5s",
  });
  check(res, { "status is 200": (r) => r.status === 200 });
}
