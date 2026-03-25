// Package calendar provides ICS calendar fetching and parsing for Immich Kiosk.
package calendar

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/log/v2"
)

// Event represents a single calendar event.
type Event struct {
	Title    string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
}

var (
	mu           sync.RWMutex
	cachedEvents []Event

	httpTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	httpClient = &http.Client{
		Transport: httpTransport,
		Timeout:   30 * time.Second,
	}
)

// StartUpdates fetches the ICS calendar immediately and then refreshes it every updateInterval seconds.
func StartUpdates(ctx context.Context, calURL string, updateInterval int) {
	interval := time.Duration(updateInterval) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	if err := fetchAndCache(ctx, calURL); err != nil {
		log.Error("Initial calendar fetch failed", "err", err)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := fetchAndCache(ctx, calURL); err != nil {
					log.Warn("Calendar refresh failed", "err", err)
				}
			}
		}
	}()
}

func fetchAndCache(ctx context.Context, calURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, calURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "immich-kiosk")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching ICS: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	events, err := parseICS(string(body))
	if err != nil {
		return fmt.Errorf("parsing ICS: %w", err)
	}

	mu.Lock()
	cachedEvents = events
	mu.Unlock()

	log.Debug("Calendar updated", "total_events", len(events))
	return nil
}

// UpcomingEvents returns events within the next daysAhead days, sorted by start time.
// maxEvents limits the number returned (0 = no limit).
func UpcomingEvents(maxEvents, daysAhead int) []Event {
	mu.RLock()
	all := make([]Event, len(cachedEvents))
	copy(all, cachedEvents)
	mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(time.Duration(daysAhead) * 24 * time.Hour)

	var upcoming []Event
	for _, e := range all {
		end := e.End
		if end.IsZero() {
			if e.AllDay {
				end = e.Start.Add(24 * time.Hour)
			} else {
				end = e.Start.Add(time.Hour)
			}
		}
		if end.After(now) && e.Start.Before(cutoff) {
			upcoming = append(upcoming, e)
		}
	}

	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].Start.Before(upcoming[j].Start)
	})

	if maxEvents > 0 && len(upcoming) > maxEvents {
		upcoming = upcoming[:maxEvents]
	}

	return upcoming
}

// parseICS parses an iCalendar string and returns all VEVENT entries.
func parseICS(content string) ([]Event, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := unfoldLines(content)

	var events []Event
	var inEvent bool
	var (
		summary      string
		location     string
		dtstart      string
		dtend        string
		dtstartParam string
		dtendParam   string
	)

	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		switch line {
		case "BEGIN:VEVENT":
			inEvent = true
			summary, location = "", ""
			dtstart, dtend = "", ""
			dtstartParam, dtendParam = "", ""
		case "END:VEVENT":
			if inEvent && dtstart != "" {
				start, allDay, err := parseDatetime(dtstart, dtstartParam)
				if err == nil {
					ev := Event{
						Title:    summary,
						Start:    start,
						AllDay:   allDay,
						Location: location,
					}
					if dtend != "" {
						if end, _, err := parseDatetime(dtend, dtendParam); err == nil {
							ev.End = end
						}
					}
					if ev.Title == "" {
						ev.Title = "(No title)"
					}
					events = append(events, ev)
				}
			}
			inEvent = false
		default:
			if !inEvent {
				continue
			}
			prop, params, value := splitProperty(line)
			switch prop {
			case "SUMMARY":
				summary = unescapeICS(value)
			case "LOCATION":
				location = unescapeICS(value)
			case "DTSTART":
				dtstart = value
				dtstartParam = params
			case "DTEND":
				dtend = value
				dtendParam = params
			}
		}
	}

	return events, nil
}

// unfoldLines joins continuation lines per RFC 5545 (line folding).
func unfoldLines(content string) []string {
	rawLines := strings.Split(content, "\n")
	var lines []string
	for i := 0; i < len(rawLines); i++ {
		line := rawLines[i]
		for i+1 < len(rawLines) {
			next := rawLines[i+1]
			if len(next) > 0 && (next[0] == ' ' || next[0] == '\t') {
				line += next[1:]
				i++
			} else {
				break
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// splitProperty splits "PROPNAME[;PARAMS]:VALUE" into (propname, params, value).
func splitProperty(line string) (string, string, string) {
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return "", "", ""
	}
	nameAndParams := line[:colonIdx]
	value := line[colonIdx+1:]
	parts := strings.SplitN(nameAndParams, ";", 2)
	prop := strings.ToUpper(parts[0])
	params := ""
	if len(parts) > 1 {
		params = parts[1]
	}
	return prop, params, value
}

// parseDatetime parses an ICS datetime value.
// params is the params string, e.g. "TZID=Europe/Rome" or "VALUE=DATE".
func parseDatetime(value, params string) (time.Time, bool, error) {
	upperParams := strings.ToUpper(params)

	// All-day: VALUE=DATE or 8-digit string without T
	if strings.Contains(upperParams, "VALUE=DATE") || (len(value) == 8 && !strings.Contains(value, "T")) {
		t, err := time.Parse("20060102", value)
		return t, true, err
	}

	// UTC (ends with Z)
	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		return t, false, err
	}

	// Timezone-aware
	if tzid := extractTZID(params); tzid != "" {
		loc, err := time.LoadLocation(tzid)
		if err != nil {
			loc = time.Local
		}
		t, err := time.ParseInLocation("20060102T150405", value, loc)
		return t, false, err
	}

	// Floating (no timezone) — treat as local time
	t, err := time.ParseInLocation("20060102T150405", value, time.Local)
	return t, false, err
}

// extractTZID extracts the TZID value from a params string like "TZID=Europe/Rome".
func extractTZID(params string) string {
	for _, part := range strings.Split(params, ";") {
		if strings.HasPrefix(strings.ToUpper(part), "TZID=") {
			tzid := part[5:]
			return strings.Trim(tzid, "\"'")
		}
	}
	return ""
}

// unescapeICS unescapes ICS text property values per RFC 5545.
func unescapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\n", " ")
	s = strings.ReplaceAll(s, "\\N", " ")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
