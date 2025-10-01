package helpers

import "time"

// ParseDateTime parses a date and a time from a string.
func ParseDateTime(dateString, timeString string) (time.Time, error) {
	// Parse date
	d, err := ParseDate(dateString)
	if err != nil {
		return time.Time{}, err
	}

	// Parse time
	t, err := ParseTime(timeString)
	if err != nil {
		return time.Time{}, err
	}

	// Combine date and time
	return time.Date(
		d.Year(),
		d.Month(),
		d.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
		t.UTC().Location(),
	), nil
}

// ParseDate parses a date from a string.
func ParseDate(date string) (time.Time, error) {
	// Parse date
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, err
	}

	return d, nil
}

// ParseTime parses a time from a string.
func ParseTime(timeString string) (time.Time, error) {
	switch len(timeString) {
	case 5:
		timeString += ":00"
	case 2:
		timeString += ":00:00"
	}

	// Parse time
	t, err := time.Parse("15:04:05", timeString)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}
