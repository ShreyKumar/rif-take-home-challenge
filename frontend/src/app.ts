// RIF Mutant Detector — frontend logic.
// Talks to the Go backend at the same origin:
//   POST /mutant/  -> 200 mutant · 403 human · 400 invalid · 405 non-POST
//   GET  /stats/   -> { count_mutant_dna, count_human_dna, ratio }
// The HTTP STATUS CODE is the authoritative contract; the JSON body is advisory.

type ResultState = "positive" | "neutral" | "error" | "info";

interface ErrorBody {
  error?: string;
}

interface StatsResponse {
  count_mutant_dna: number;
  count_human_dna: number;
  ratio: number;
}

function requireElement<T extends HTMLElement>(id: string): T {
  const el = document.getElementById(id);
  if (!el) {
    throw new Error(`Missing required element: #${id}`);
  }
  return el as T;
}

const form = requireElement<HTMLFormElement>("dna-form");
const textarea = requireElement<HTMLTextAreaElement>("dna");
const checkBtn = requireElement<HTMLButtonElement>("check-btn");
const resultEl = requireElement<HTMLElement>("result");

const refreshBtn = requireElement<HTMLButtonElement>("refresh-stats-btn");
const statMutant = requireElement<HTMLElement>("stat-mutant");
const statHuman = requireElement<HTMLElement>("stat-human");
const statRatio = requireElement<HTMLElement>("stat-ratio");
const statsError = requireElement<HTMLElement>("stats-error");

// Parse the textarea into an array of trimmed, non-empty, upper-cased rows.
function parseDna(raw: string): string[] {
  return raw
    .split("\n")
    .map((line) => line.trim().toUpperCase())
    .filter((line) => line.length > 0);
}

// Show a message in the result region with a given visual state.
function showResult(message: string, state: ResultState = "info"): void {
  resultEl.textContent = message;
  resultEl.className = `result show ${state}`;
}

// Try to extract a human-readable error message from a response body.
function extractError(body: unknown): string | null {
  if (body && typeof body === "object" && typeof (body as ErrorBody).error === "string") {
    return (body as ErrorBody).error as string;
  }
  return null;
}

// Read a response body as JSON, tolerating empty or non-JSON payloads.
async function readJson(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) {
    return null;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

async function handleSubmit(event: SubmitEvent): Promise<void> {
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
    // Refresh stats after a successful check so the panel stays current.
    void loadStats();
  }
}

// Format the ratio to a few decimal places; guard against non-numbers.
function formatRatio(value: number): string {
  const n = Number(value);
  if (!isFinite(n)) {
    return "—";
  }
  return n.toFixed(3);
}

async function loadStats(): Promise<void> {
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
    const data = (await response.json()) as StatsResponse;
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
