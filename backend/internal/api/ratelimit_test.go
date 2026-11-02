package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRateLimiterWindow(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(3, time.Minute)
	rl.now = func() time.Time { return now }
	id := testUserA
	for i := 0; i < 3; i++ {
		if !rl.allow(id) {
			t.Fatalf("request %d blocked", i)
		}
	}
	if rl.allow(id) {
		t.Fatal("4th request allowed")
	}
	now = now.Add(2 * time.Minute) // window slides
	if !rl.allow(id) {
		t.Fatal("old requests not expired")
	}
}

func TestUploadLimitTenPerHour(t *testing.T) {
	if uploadLimit.max != 10 || uploadLimit.period != time.Hour {
		t.Fatalf("upload limit = %d per %s; want 10 per hour", uploadLimit.max, uploadLimit.period)
	}
	now := time.Now()
	rl := newRateLimiter(10, time.Hour)
	rl.now = func() time.Time { return now }
	for i := 0; i < 10; i++ {
		if !rl.allow(testUserA) {
			t.Fatalf("upload %d rejected", i+1)
		}
	}
	if rl.allow(testUserA) {
		t.Fatal("11th upload allowed within the hour")
	}
	now = now.Add(time.Hour)
	if !rl.allow(testUserA) {
		t.Fatal("upload blocked after the hour expired")
	}
}

func TestOpsRateLimit429(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	tex, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Skip("fixture missing")
	}
	body, ct := uploadBody(t, "resume.tex", string(tex))
	req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d", rec.Code)
	}
	for i := 0; i < 30; i++ {
		opsLimit.allow(testUserA)
	}
	payload := bytes.NewReader([]byte(`{"revision":0,"ops":[{"op":"edit","id":"b_x","value":"y"}]}`))
	req = httptest.NewRequest(http.MethodPost, "/api/drafts/"+uuid_()+"/ops", payload)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
}

func uuid_() string { return testUserA.String() }
