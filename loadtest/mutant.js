// k6 load script (documented alternative to loadgen — requires k6 installed).
//
//   k6 run -e BASE_URL=http://127.0.0.1:8080 loadtest/mutant.js
//
// Mirrors loadgen's mixed workload: ~90% POST /mutant/ (mutant + human) and
// ~10% GET /stats/. Ramps virtual users to exercise several concurrency levels.
import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:8080";

// k6's http_req_failed counts anything outside 200–399 as a failure by
// default, but 403 is the expected "human" verdict here (~40% of requests) —
// without this, the threshold below would trip on every run against a
// perfectly healthy server.
http.setResponseCallback(http.expectedStatuses(200, 403));

const MUTANT = JSON.stringify({
  dna: ["ATGCGA", "CAGTGC", "TTATGT", "AGAAGG", "CCCCTA", "TCACTG"],
});
const HUMAN = JSON.stringify({ dna: ["ATGC", "GCAT", "TACG", "CGTA"] });
const JSON_HEADERS = { headers: { "Content-Type": "application/json" } };

export const options = {
  stages: [
    { duration: "10s", target: 8 },
    { duration: "10s", target: 32 },
    { duration: "10s", target: 64 },
    { duration: "10s", target: 128 },
    { duration: "5s", target: 0 },
  ],
  thresholds: {
    // No 5xx: mutant=200 and human=403 are both expected, non-error answers.
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const r = __ITER % 10;
  if (r === 9) {
    const res = http.get(`${BASE_URL}/stats/`);
    check(res, { "stats 200": (x) => x.status === 200 });
  } else {
    const body = r % 2 === 0 ? MUTANT : HUMAN;
    const res = http.post(`${BASE_URL}/mutant/`, body, JSON_HEADERS);
    check(res, { "mutant 200/403": (x) => x.status === 200 || x.status === 403 });
  }
  sleep(0.001);
}
