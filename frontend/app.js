// RIF Mutant Detector — frontend logic.
// Talks to the Go backend at the same origin:
//   POST /mutant/  -> 200 mutant · 403 human · 400 invalid · 405 non-POST
//   GET  /stats/   -> { count_mutant_dna, count_human_dna, ratio }
// The HTTP STATUS CODE is the authoritative contract; the JSON body is advisory.

(function () {
  "use strict";

  var form = document.getElementById("dna-form");
  var textarea = document.getElementById("dna");
  var checkBtn = document.getElementById("check-btn");
  var resultEl = document.getElementById("result");

  var refreshBtn = document.getElementById("refresh-stats-btn");
  var statMutant = document.getElementById("stat-mutant");
  var statHuman = document.getElementById("stat-human");
  var statRatio = document.getElementById("stat-ratio");
  var statsError = document.getElementById("stats-error");

  // Parse the textarea into an array of trimmed, non-empty, upper-cased rows.
  function parseDna(raw) {
    return raw
      .split("\n")
      .map(function (line) {
        return line.trim().toUpperCase();
      })
      .filter(function (line) {
        return line.length > 0;
      });
  }

  // Show a message in the result region with a given visual state.
  // state is one of: "positive", "neutral", "error", "info".
  function showResult(message, state) {
    resultEl.textContent = message;
    resultEl.className = "result show " + (state || "info");
  }

  // Try to extract a human-readable error message from a response body.
  function extractError(body) {
    if (body && typeof body === "object" && typeof body.error === "string") {
      return body.error;
    }
    return null;
  }

  // Read a response body as JSON, tolerating empty or non-JSON payloads.
  function readJson(response) {
    return response.text().then(function (text) {
      if (!text) {
        return null;
      }
      try {
        return JSON.parse(text);
      } catch (e) {
        return null;
      }
    });
  }

  function handleSubmit(event) {
    event.preventDefault();

    var rows = parseDna(textarea.value);

    // Minimal client-side check only — the server is authoritative on validity.
    if (rows.length === 0) {
      showResult("Please enter at least one row of DNA (one sequence per line).", "error");
      return;
    }

    checkBtn.disabled = true;
    showResult("Checking…", "info");

    fetch("/mutant/", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dna: rows })
    })
      .then(function (response) {
        return readJson(response).then(function (body) {
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
                extractError(body) ||
                  "Unexpected server response (HTTP " + response.status + "). Please try again.",
                "error"
              );
          }
        });
      })
      .catch(function () {
        showResult("Could not reach the server. Check your connection and try again.", "error");
      })
      .then(function () {
        checkBtn.disabled = false;
        // Refresh stats after a successful check so the panel stays current.
        loadStats();
      });
  }

  // Format the ratio to a few decimal places; guard against non-numbers.
  function formatRatio(value) {
    var n = Number(value);
    if (!isFinite(n)) {
      return "—";
    }
    return n.toFixed(3);
  }

  function loadStats() {
    statsError.textContent = "";
    refreshBtn.disabled = true;

    fetch("/stats/", {
      method: "GET",
      headers: { Accept: "application/json" }
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        return response.json();
      })
      .then(function (data) {
        statMutant.textContent = String(data.count_mutant_dna);
        statHuman.textContent = String(data.count_human_dna);
        statRatio.textContent = formatRatio(data.ratio);
      })
      .catch(function () {
        statsError.textContent = "Could not load statistics right now.";
      })
      .then(function () {
        refreshBtn.disabled = false;
      });
  }

  form.addEventListener("submit", handleSubmit);
  refreshBtn.addEventListener("click", loadStats);

  // Load stats once on page load.
  loadStats();
})();
