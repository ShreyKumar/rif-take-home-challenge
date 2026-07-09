package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
)

// recordingStore is a fake Store that captures saved records; its name is
// scoped to this file to avoid colliding with the /stats/ handler's fake.
type recordingStore struct {
	saved   []contract.Record
	saveErr error
}

func (s *recordingStore) Save(rec contract.Record) (bool, error) {
	if s.saveErr != nil {
		return false, s.saveErr
	}
	s.saved = append(s.saved, rec)
	return true, nil
}

func (s *recordingStore) Stats() (uint64, uint64, error) { return 0, 0, nil }

// detectorReturning is a fake Detector with a fixed verdict.
func detectorReturning(v bool) contract.Detector {
	return func([]string) bool { return v }
}

func doMutant(t *testing.T, h http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/mutant/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMutantEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		verdict    bool // detector result
		wantStatus int
		wantMutant bool
		wantStored bool
	}{
		{"mutant 200", http.MethodPost, `{"dna":["ATGCGA","CAGTGC","TTATGT","AGAAGG","CCCCTA","TCACTG"]}`, true, http.StatusOK, true, true},
		{"human 403", http.MethodPost, `{"dna":["ATGC","GCAT","TACG","CGTA"]}`, false, http.StatusForbidden, false, true},
		{"small valid is human 403", http.MethodPost, `{"dna":["ATG","CAG","TTA"]}`, false, http.StatusForbidden, false, true},
		{"malformed json 400", http.MethodPost, `{"dna":`, false, http.StatusBadRequest, false, false},
		{"missing dna 400", http.MethodPost, `{}`, false, http.StatusBadRequest, false, false},
		{"empty dna 400", http.MethodPost, `{"dna":[]}`, false, http.StatusBadRequest, false, false},
		{"non-square 400", http.MethodPost, `{"dna":["ATGC","CAG"]}`, false, http.StatusBadRequest, false, false},
		{"illegal char 400", http.MethodPost, `{"dna":["ATGX","CAGT","TTAT","AGAA"]}`, false, http.StatusBadRequest, false, false},
		{"non-post 405", http.MethodGet, ``, false, http.StatusMethodNotAllowed, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingStore{}
			h := NewMutantHandler(detectorReturning(tc.verdict), store)

			rec := doMutant(t, h, tc.method, tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusOK || tc.wantStatus == http.StatusForbidden {
				var got contract.MutantResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if got.Mutant != tc.wantMutant {
					t.Fatalf("body.mutant = %v, want %v", got.Mutant, tc.wantMutant)
				}
			}

			if tc.wantStored && len(store.saved) != 1 {
				t.Fatalf("stored %d records, want 1", len(store.saved))
			}
			if !tc.wantStored && len(store.saved) != 0 {
				t.Fatalf("stored %d records, want 0 (invalid input must not persist)", len(store.saved))
			}
		})
	}
}

// TestMutantStoresJoinedDNAWithVerdict checks the record handed to the store.
func TestMutantStoresJoinedDNAWithVerdict(t *testing.T) {
	store := &recordingStore{}
	h := NewMutantHandler(detectorReturning(true), store)

	doMutant(t, h, http.MethodPost, `{"dna":["AAAA","GGGG","CCCC","TTTT"]}`)

	if len(store.saved) != 1 {
		t.Fatalf("stored %d records, want 1", len(store.saved))
	}
	got := store.saved[0]
	if want := "AAAA\nGGGG\nCCCC\nTTTT"; got.DNA != want {
		t.Fatalf("stored DNA = %q, want %q", got.DNA, want)
	}
	if !got.IsMutant {
		t.Fatal("stored record IsMutant = false, want true")
	}
}

// TestMutantStoreErrorIs500 verifies a persistence failure surfaces as 500.
func TestMutantStoreErrorIs500(t *testing.T) {
	store := &recordingStore{saveErr: errors.New("disk on fire")}
	h := NewMutantHandler(detectorReturning(true), store)

	rec := doMutant(t, h, http.MethodPost, `{"dna":["AAAA","GGGG","CCCC","TTTT"]}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// assertErrorBody asserts the response carries the {"error": <non-empty>} shape.
func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body contract.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v (body %q)", err, rec.Body.String())
	}
	if body.Error == "" {
		t.Fatalf("error body has empty \"error\" field: %q", rec.Body.String())
	}
}

// TestMutantMethodNotAllowed checks the 405 path advertises Allow: POST across
// several non-POST verbs, returns the {"error":...} body, and stores nothing.
func TestMutantMethodNotAllowed(t *testing.T) {
	store := &recordingStore{}
	h := NewMutantHandler(detectorReturning(false), store)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := doMutant(t, h, method, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
		if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("%s: Allow = %q, want %q", method, allow, http.MethodPost)
		}
		assertErrorBody(t, rec)
	}
	if len(store.saved) != 0 {
		t.Fatalf("non-POST requests stored %d records, want 0", len(store.saved))
	}
}

// TestMutant400ReturnsErrorBody pins the error-body contract on the 400 path.
func TestMutant400ReturnsErrorBody(t *testing.T) {
	store := &recordingStore{}
	h := NewMutantHandler(detectorReturning(false), store)

	rec := doMutant(t, h, http.MethodPost, `{"dna":["ATGC","CAG"]}`) // non-square
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorBody(t, rec)
}

// TestMutantTrailingDataRejected verifies a body that is a valid object
// followed by junk is rejected as malformed (FR-2.4) and not persisted.
func TestMutantTrailingDataRejected(t *testing.T) {
	store := &recordingStore{}
	h := NewMutantHandler(detectorReturning(false), store)

	rec := doMutant(t, h, http.MethodPost, `{"dna":["ATGC","GCAT","TACG","CGTA"]}garbage`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorBody(t, rec)
	if len(store.saved) != 0 {
		t.Fatalf("trailing-data body stored %d records, want 0", len(store.saved))
	}
}

// TestMutantBodyTooLarge verifies an oversized body is rejected before it is
// fully buffered, via http.MaxBytesReader, and is not persisted.
func TestMutantBodyTooLarge(t *testing.T) {
	store := &recordingStore{}
	h := NewMutantHandler(detectorReturning(true), store)

	huge := strings.Repeat("A", maxBodyBytes+1024)
	rec := doMutant(t, h, http.MethodPost, `{"dna":["`+huge+`"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for oversized body", rec.Code, http.StatusBadRequest)
	}
	assertErrorBody(t, rec)
	if len(store.saved) != 0 {
		t.Fatalf("oversized body stored %d records, want 0", len(store.saved))
	}
}

// TestMutantPassesParsedDNAToDetector asserts the handler forwards the parsed
// rows to the injected detector (not covered by the fixed-verdict fake).
func TestMutantPassesParsedDNAToDetector(t *testing.T) {
	var got []string
	detect := func(dna []string) bool { got = dna; return false }
	h := NewMutantHandler(detect, &recordingStore{})

	doMutant(t, h, http.MethodPost, `{"dna":["ATGC","GCAT","TACG","CGTA"]}`)

	if want := "ATGC,GCAT,TACG,CGTA"; strings.Join(got, ",") != want {
		t.Fatalf("detector received %q, want rows %q", got, want)
	}
}
