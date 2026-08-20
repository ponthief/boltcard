package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func Test_concurrency_limiter_allows_up_to_the_cap(t *testing.T) {
	c := New_concurrency_limiter(2)

	// held blocks inside the handler, so the slots stay taken
	release := make(chan struct{})
	entered := make(chan struct{}, 3)

	handler := c.Middleware(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	codes := make([]int, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest("GET", "/ln", nil))
			codes[i] = w.Code
		}(i)
	}

	// wait for both to be inside the handler
	<-entered
	<-entered

	if got := c.In_flight(); got != 2 {
		t.Errorf("in flight = %d, want 2", got)
	}

	// a third request has no slot to take
	third := httptest.NewRecorder()
	handler(third, httptest.NewRequest("GET", "/ln", nil))

	if third.Code != http.StatusServiceUnavailable {
		t.Errorf("third request status = %d, want %d", third.Code, http.StatusServiceUnavailable)
	}

	close(release)
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d status = %d, want %d", i+1, code, http.StatusOK)
		}
	}

	if got := c.In_flight(); got != 0 {
		t.Errorf("in flight after completion = %d, want 0", got)
	}
}

func Test_concurrency_limiter_releases_slots_after_a_panic(t *testing.T) {
	c := New_concurrency_limiter(1)

	handler := c.Middleware(func(w http.ResponseWriter, r *http.Request) {
		panic("handler failed")
	})

	func() {
		defer func() { recover() }()
		handler(httptest.NewRecorder(), httptest.NewRequest("GET", "/ln", nil))
	}()

	if got := c.In_flight(); got != 0 {
		t.Fatalf("in flight after a panic = %d, want 0", got)
	}

	// the slot is usable again
	ok_handler := c.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	ok_handler(w, httptest.NewRequest("GET", "/ln", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func Test_concurrency_limiter_minimum_of_one(t *testing.T) {
	c := New_concurrency_limiter(0)

	handler := c.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/ln", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
