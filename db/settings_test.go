package db

import (
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_a_held_value_is_reused(t *testing.T) {
	reads := 0
	now := time.Now()

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads++
		return "DEBUG", nil
	})
	c.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		if got := c.get("LOG_LEVEL"); got != "DEBUG" {
			t.Fatalf("get() = %q, want DEBUG", got)
		}
	}

	if reads != 1 {
		t.Errorf("database reads = %d, want 1", reads)
	}
}

func Test_a_held_value_expires(t *testing.T) {
	reads := 0
	now := time.Now()

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads++
		return "DEBUG", nil
	})
	c.now = func() time.Time { return now }

	c.get("LOG_LEVEL")
	now = now.Add(9 * time.Second)
	c.get("LOG_LEVEL")

	if reads != 1 {
		t.Fatalf("database reads before expiry = %d, want 1", reads)
	}

	now = now.Add(2 * time.Second)
	c.get("LOG_LEVEL")

	if reads != 2 {
		t.Errorf("database reads after expiry = %d, want 2", reads)
	}
}

func Test_settings_are_held_separately(t *testing.T) {
	reads := map[string]int{}

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads[name]++
		return name + "_value", nil
	})

	c.get("LN_HOST")
	c.get("LN_PORT")
	c.get("LN_HOST")

	if reads["LN_HOST"] != 1 || reads["LN_PORT"] != 1 {
		t.Errorf("reads = %v, want one of each", reads)
	}

	if got := c.get("LN_PORT"); got != "LN_PORT_value" {
		t.Errorf("get(LN_PORT) = %q", got)
	}
}

// a setting with no row is a settled answer, so it is held
func Test_a_missing_setting_is_held(t *testing.T) {
	reads := 0

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads++
		return "", sql.ErrNoRows
	})

	if got := c.get("NOT_SET"); got != "" {
		t.Errorf("get() = %q, want an empty string", got)
	}

	c.get("NOT_SET")

	if reads != 1 {
		t.Errorf("database reads = %d, want 1", reads)
	}
}

// a database that cannot be reached must not leave a setting reading as unset
func Test_a_failed_read_is_not_held(t *testing.T) {
	reads := 0
	fail := true

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads++
		if fail {
			return "", errors.New("connection refused")
		}
		return "ENABLE", nil
	})

	if got := c.get("FUNCTION_LNURLW"); got != "" {
		t.Errorf("get() during an outage = %q, want an empty string", got)
	}

	fail = false

	if got := c.get("FUNCTION_LNURLW"); got != "ENABLE" {
		t.Errorf("get() after the outage = %q, want ENABLE", got)
	}

	if reads != 2 {
		t.Errorf("database reads = %d, want 2", reads)
	}
}

func Test_a_zero_cache_period_reads_every_time(t *testing.T) {
	reads := 0

	c := new_setting_cache(0, func(name string) (string, error) {
		reads++
		return "DEBUG", nil
	})

	for i := 0; i < 4; i++ {
		c.get("LOG_LEVEL")
	}

	if reads != 4 {
		t.Errorf("database reads = %d, want 4", reads)
	}
}

func Test_clear_forces_a_read(t *testing.T) {
	reads := 0

	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		reads++
		return "DEBUG", nil
	})

	c.get("LOG_LEVEL")
	c.clear()
	c.get("LOG_LEVEL")

	if reads != 2 {
		t.Errorf("database reads = %d, want 2", reads)
	}
}

// settings are read from many request goroutines at once
func Test_concurrent_use(t *testing.T) {
	c := new_setting_cache(10*time.Second, func(name string) (string, error) {
		return "value", nil
	})

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if got := c.get("LN_HOST"); got != "value" {
					t.Errorf("get() = %q, want value", got)
					return
				}
			}
		}()
	}

	wg.Wait()
}
