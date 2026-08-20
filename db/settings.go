package db

import (
	"database/sql"
	"sync"
	"time"
)

// Default_setting_cache_sec is how long a setting value is reused before it is
// read from the database again.
//
// Settings are read on every request, several times over, and each read opened
// its own connection to the database. Reusing a value for a few seconds takes
// almost all of that load off the database. The cost is that a change to a
// setting takes up to this long to come into effect.
const Default_setting_cache_sec = 10

type setting_entry struct {
	value      string
	fetched_at time.Time
}

type setting_cache struct {
	mutex   sync.RWMutex
	entries map[string]setting_entry
	ttl     time.Duration
	// these are fields so that tests need no database and no sleeping
	now   func() time.Time
	fetch func(string) (string, error)
}

var settings = new_setting_cache(Default_setting_cache_sec*time.Second, read_setting)

func new_setting_cache(ttl time.Duration, fetch func(string) (string, error)) *setting_cache {
	return &setting_cache{
		entries: make(map[string]setting_entry),
		ttl:     ttl,
		now:     time.Now,
		fetch:   fetch,
	}
}

// get returns a setting value, reading it from the database when there is no
// value held for it or the value held has expired.
//
// A value is only held when it was read successfully. A failure to reach the
// database is not held, so that a setting does not read as unset for the life
// of an entry because the database was briefly unavailable.
func (c *setting_cache) get(setting_name string) string {
	if c.ttl > 0 {
		c.mutex.RLock()
		entry, found := c.entries[setting_name]
		c.mutex.RUnlock()

		if found && c.now().Sub(entry.fetched_at) < c.ttl {
			return entry.value
		}
	}

	value, err := c.fetch(setting_name)

	// a setting with no row is a settled answer and is held; any other error
	// is not
	if err != nil && err != sql.ErrNoRows {
		return ""
	}

	if c.ttl > 0 {
		c.mutex.Lock()
		c.entries[setting_name] = setting_entry{value: value, fetched_at: c.now()}
		c.mutex.Unlock()
	}

	return value
}

func (c *setting_cache) clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]setting_entry)
}

// read_setting reads one setting value from the database.
func read_setting(setting_name string) (string, error) {

	setting_value := ""

	db, err := open()
	if err != nil {
		return "", err
	}
	defer db.Close()

	sqlStatement := `select value from settings where name=$1;`

	row := db.QueryRow(sqlStatement, setting_name)
	err = row.Scan(&setting_value)
	if err != nil {
		return "", err
	}

	return setting_value, nil
}

// Get_setting returns a setting value, which may have been read from the
// database up to the cache period ago. It returns an empty string for a
// setting that is not set or could not be read.
func Get_setting(setting_name string) string {
	return settings.get(setting_name)
}

// Get_setting_now returns a setting value read from the database, ignoring and
// refreshing any value held for it. Use it where a stale value would be wrong,
// such as when reading the cache period itself at start up.
func Get_setting_now(setting_name string) string {
	settings.clear()

	value, err := read_setting(setting_name)
	if err != nil {
		return ""
	}

	return value
}

// Set_setting_cache_seconds sets how long setting values are reused.
// Zero turns off reuse, so that every read goes to the database.
func Set_setting_cache_seconds(seconds int) {
	if seconds < 0 {
		seconds = 0
	}

	settings.mutex.Lock()
	settings.ttl = time.Duration(seconds) * time.Second
	settings.entries = make(map[string]setting_entry)
	settings.mutex.Unlock()
}
