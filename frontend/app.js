// RIF Mutant Detector — frontend logic (vanilla JS, no build step).
// Talks to the Go backend at the same origin:
//   POST /mutant/  -> 200 mutant · 403 human · 400 invalid · 405 non-POST
//   GET  /stats/   -> { count_mutant_dna, count_human_dna, ratio }
// The HTTP STATUS CODE is the authoritative contract; the JSON body is advisory.

function requireElement(id) {
  const el = document.getElementById(id);
  if (!el) {
    throw new Error(`Missing required element: #${id}`);
  }
  return el;
}

const form = requireElement("dna-form");
const textarea = requireElement("dna");
const checkBtn = requireElement("check-btn");
const resultEl = requireElement("result");

const refreshBtn = requireElement("refresh-stats-btn");
const statMutant = requireElement("stat-mutant");
const statHuman = requireElement("stat-human");
const statRatio = requireElement("stat-ratio");
const statsError = requireElement("stats-error");

// Parse the textarea into an array of trimmed, non-empty, upper-cased rows.
function parseDna(raw) {
  return raw
    .split("\n")
    .map((line) => line.trim().toUpperCase())
    .filter((line) => line.length > 0);
}

// Show a message in the result region with a given visual state.
function showResult(message, state = "info") {
  resultEl.textContent = message;
  resultEl.className = `result show ${state}`;
}

// Try to extract a human-readable error message from a response body.
function extractError(body) {
  if (body && typeof body === "object" && typeof body.error === "string") {
    return body.error;
  }
  return null;
}

// Read a response body as JSON, tolerating empty or non-JSON payloads.
async function readJson(response) {
  const text = await response.text();
  if (!text) {
    return null;
  }
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function handleSubmit(event) {
  event.preventDefault();

  const rows = parseDna(textarea.value);

  // Minimal client-side check only — the server is authoritative on validity.
  if (rows.length === 0) {
    showResult("Please enter at least one row of DNA (one sequence per line).", "error");
    return;
  }

  checkBtn.disabled = true;
  showResult("Checking…", "info");

  try {
    const response = await fetch("/mutant/", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dna: rows })
    });
    const body = await readJson(response);

    switch (response.status) {
      case 200:
        showResult("Mutant detected. This DNA belongs to a mutant.", "positive");
        break;
      case 403:
        showResult("Human. No mutant signature found in this DNA.", "neutral");
        break;
      case 400:
        showResult(
          extractError(body) || "Invalid input. Check that the matrix is square and uses only A, T, C, G.",
          "error"
        );
        break;
      default:
        showResult(
          extractError(body) || `Unexpected server response (HTTP ${response.status}). Please try again.`,
          "error"
        );
    }
  } catch {
    showResult("Could not reach the server. Check your connection and try again.", "error");
  } finally {
    checkBtn.disabled = false;
    // Refresh stats after a check so the panel stays current.
    void loadStats();
  }
}

// Format the ratio to a few decimal places; guard against non-numbers.
function formatRatio(value) {
  const n = Number(value);
  if (!isFinite(n)) {
    return "—";
  }
  return n.toFixed(3);
}

async function loadStats() {
  statsError.textContent = "";
  refreshBtn.disabled = true;

  try {
    const response = await fetch("/stats/", {
      method: "GET",
      headers: { Accept: "application/json" }
    });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const data = await response.json();
    statMutant.textContent = String(data.count_mutant_dna);
    statHuman.textContent = String(data.count_human_dna);
    statRatio.textContent = formatRatio(data.ratio);
  } catch {
    statsError.textContent = "Could not load statistics right now.";
  } finally {
    refreshBtn.disabled = false;
  }
}

form.addEventListener("submit", (event) => {
  void handleSubmit(event);
});
refreshBtn.addEventListener("click", () => {
  void loadStats();
});

// Load stats once on page load.
void loadStats();
