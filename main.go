package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"bytes"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"os"
)

/*

              __                  __
             ( _)                ( _)
            / / \\              / /\_\_
           / /   \\            / / | \ \
          / /     \\          / /  |\ \ \
         /  /   ,  \ ,       / /   /|  \ \
        /  /    |\_ /|      / /   / \   \_\
       /  /  |\/ _ '_|\    / /   /   \    \\         ! Here be dragons !
      |  /   |/  0 \0\ \  / |    |    \    \\        ------------------
      |    |\|      \_\_ /  /    |     \    \\       Warning: Hack-y, lazy-weekend, "it-works", type of code curry ahead
      |  | |/    \.\ o\o)  /      \     |    \\      @sogko
      \    |     /\\`v-v  /        |    |     \\
       | \/    /_| \\_|  /         |    | \    \\
       | |    /__/_     /   _____  |    |  \    \\
       \|    [__]  \_/  |_________  \   |   \    ()
        /    [___] (    \         \  |\ |   |   //
       |    [___]                  |\| \|   /  |/
      /|    [____]                  \  |/\ / / ||
     (  \   [____ /     ) _\      \  \    \| | ||
      \  \  [_____|    / /     __/    \   / / //
      |   \ [_____/   / /        \    |   \/ //
      |   /  '----|   /=\____   _/    |   / //
   __ /  /        |  /   ___/  _/\    \  | ||
  (/-(/-\)       /   \  (/\/\)/  |    /  | /
                (/\/\)           /   /   //
                       _________/   /    /
                      \____________/    (


*/

const SessionsJSONURL = "https://raw.githubusercontent.com/fossasia/open-event-scraper/master/out/sessions.json"
const ServiceKeyFilename = "service_key.json"
const CalendarDataFilename = "data.json"
const GoogleCalendarURLBase = "https://calendar.google.com/calendar/render"
const DefaultLocation = "Misc"

var ScrubbedLocation = map[string]string{
	"Exhibition and Snack Area":                          "Exhibition and Snack Area",
	"Level 3, Dalton Hall":                               "Level 3, Dalton Hall",
	"Level 3, Level 3, Dalton Hall":                      "Level 3, Dalton Hall",
	"Level 1, Observatory Room":                          "Level 1, Observatory Room",
	"Level 3, Faraday Lab":                               "Level 3, Faraday Lab",
	"Level 3, Planck Lab":                                "Level 3, Planck Lab",
	"Level 2, Room to be decided":                        "Level 2, Room to be decided",
	"Level 3, Fermi Lab":                                 "Level 3, Fermi Lab",
	"Level 2, Herschel Lab":                              "Level 2, Herschel Lab",
	"Clarke Quay":                                        "Level 1, Ground Floor, Exhibition Hall",
	"Level 1, Digital Design Studio":                     "Level 1, Digital Design Studio",
	"Level 2, Einstein Room":                             "Level 2, Einstein Room",
	"Level 2, Einstein Hall":                             "Level 2, Einstein Room",
	"Level 1, Tinkering Studio":                          "Level 1, Tinkering Studio",
	"Level 1, Ground Floor, Exhibition Hall":             "Level 1, Ground Floor, Exhibition Hall",
	"Level 1, Ground Floor, Exhibition Hall A":           "Level 1, Ground Floor, Exhibition Hall",
	"Marquee Theatre":                                    "Marquee Theatre",
	"Dalton Hall":                                        "Level 3, Dalton Hall",
	"Level 3, Lewis Lab":                                 "Level 3, Lewis Lab",
	"Level 3, Pauling Lab":                               "Level 3, Pauling Lab",
	"Level 1, Eco Garden Lab":                            "Level 1, Eco Garden Lab",
	"Level 1, Ground Floor, Scientist For a day, Hall A": "Level 1, Ground Floor, Scientist For a day, Hall A",
}

type Speaker struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Organisation string `json:"organisation"`
}
type Track struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SessionEntry struct {
	PyObject    string    `json:"py/object"`
	Speakers    []Speaker `json:"speakers"`
	Description string    `json:"description"`
	Title       string    `json:"title"`
	Track       Track     `json:"track"`
	StartTime   string    `json:"start_time"`
	SessionID   string    `json:"session_id"`
	Location    string    `json:"location"`
	Type        string    `json:"type"`
	EndTime     string    `json:"end_time"`
}
type Speakers []Speaker

func (s Speakers) String() string {
	speakers := []Speaker(s)
	res := []string{}
	for _, speaker := range speakers {
		if speaker.Organisation != "" {
			res = append(res, fmt.Sprintf(`%v (%v)`, speaker.Name, speaker.Organisation))
		} else {
			res = append(res, fmt.Sprintf(`%v`, speaker.Name))
		}
	}
	return strings.Join(res, ", ")
}

type FOSSAsiaEvent struct {
	Sessions []*SessionEntry `json:"sessions"`
}

type AppData struct {
	MasterCalendarID           string            `json:"master_calendar_id"`
	TrackCalendarIDs           map[string]string `json:"track_calendar_ids"`
	LocationCalendarIDs        map[string]string `json:"location_calendar_ids"`
	MasterCalendarSessionIDs   map[string]string `json:"master_calendar_session_ids"`
	TrackCalendarSessionIDs    map[string]string `json:"track_calendar_session_ids"`
	LocationCalendarSessionIDs map[string]string `json:"location_calendar_session_ids"`
	MasterCalendarURL          string            `json:"master_calendar_url"`
	TrackCalendarURL           string            `json:"track_calendar_url"`
	LocationCalendarURL        string            `json:"location_calendar_url"`
}

func (d *AppData) GetMasterCalendarURL() string {
	url := []string{GoogleCalendarURLBase, "?", d.MasterCalendarID}
	d.MasterCalendarURL = strings.Join(url, "")
	return d.MasterCalendarURL
}
func (d *AppData) GetTrackCalendarURL() string {
	url := []string{GoogleCalendarURLBase, "?"}
	for _, calendarID := range d.TrackCalendarIDs {
		url = append(url, fmt.Sprintf("cid=%v&", calendarID))
	}
	d.TrackCalendarURL = strings.Join(url, "")
	return d.TrackCalendarURL
}
func (d *AppData) GetLocationCalendarURL() string {
	url := []string{GoogleCalendarURLBase, "?"}
	for _, calendarID := range d.LocationCalendarIDs {
		url = append(url, fmt.Sprintf("cid=%v&", calendarID))
	}
	d.LocationCalendarURL = strings.Join(url, "")
	return d.LocationCalendarURL
}

func createCalendar(srv *calendar.Service, summary, description string) (string, error) {
	newCal, err := srv.Calendars.Insert(&calendar.Calendar{
		Description: description,
		Summary:     summary,
		TimeZone:    "Asia/Singapore",
		Location:    "Singapore",
	}).Do()
	if err != nil {
		return "", fmt.Errorf("Failed to create master calendar %v", err)
	}

	// create ACL for master calendar
	publicACLrule := &calendar.AclRule{
		Scope: &calendar.AclRuleScope{
			Type:  "default",
			Value: "",
		},
		Role: "reader",
	}
	publicACLrule, err = srv.Acl.Insert(newCal.Id, publicACLrule).Do()
	if err != nil {
		return "", fmt.Errorf("Failed to set calendar ACL %v", err)
	}

	return newCal.Id, nil
}
func removeEntriesForDeletedSessionsFromCalendar(srv *calendar.Service, sessionIDEventIDMap map[string]string, calendarID string, sessionIDs []string) {

	// for each (session_id, event_id) pair, if the session_id does not exists in sessionIDs slice,
	// it means that the session_id has been removed
	for sessionID, eventID := range sessionIDEventIDMap {
		if eventID == "" {
			continue
		}
		if sessionID == "" {
			continue
		}
		hasSessionID := false
		for _, s := range sessionIDs {
			if sessionID == s {
				hasSessionID = true
				break
			}
		}
		if !hasSessionID {
			log.Println("Deleting event from calendar", eventID, calendarID)
			err := srv.Events.Delete(calendarID, eventID).Do()
			if err != nil {
				log.Println("Error deleting event", err)
			}
		}
	}
}

func insertOrUpdateEventForSession(srv *calendar.Service, appData *AppData, calendarType string, calendarID string, sessionID string, event *calendar.Event) {

	sessionIDEventIDMap := map[string]string{}
	switch calendarType {
	case "master":
		sessionIDEventIDMap = appData.MasterCalendarSessionIDs
	case "track":
		sessionIDEventIDMap = appData.TrackCalendarSessionIDs
	case "location":
		sessionIDEventIDMap = appData.LocationCalendarSessionIDs
	default:
		log.Println("Failed to create or update event, invalid calendar type")
		return
	}

	saveNewEventID := func(appData *AppData, calendarType string, sessionID string, eventID string) {
		if sessionID == "" {
			log.Printf("[%v]Warning: empty session ID\n", calendarID)
			return
		}
		switch calendarType {
		case "master":
			appData.MasterCalendarSessionIDs[sessionID] = eventID
		case "track":
			appData.TrackCalendarSessionIDs[sessionID] = eventID
		case "location":
			appData.LocationCalendarSessionIDs[sessionID] = eventID
		default:
			log.Println("Failed to save new event id, invalid calendar type")
			return
		}
	}

	// if current session id has an existing event id, set it so that we can update instead of destroy + create
	if eventID, ok := sessionIDEventIDMap[sessionID]; !ok || eventID == "" {
		// add new event to master calendar
		newEvent, err := srv.Events.Insert(calendarID, event).Do()
		if err != nil {
			log.Printf("[%v] Error inserting %v %v %v\n", calendarType, event.Summary, err)
		} else {
			log.Printf("[%v] Inserted %v %v\n", calendarType, newEvent.Id, newEvent.Summary)

			if sessionID != "" {
				// save event id to app data
				saveNewEventID(appData, calendarType, sessionID, newEvent.Id)
			} else {
				log.Printf("[%v] Couldn't find session ID for given event [1]\n", calendarType)
			}
		}
	} else {
		// update prev event
		newEvent, err := srv.Events.Update(calendarID, eventID, event).Do()
		if err != nil {
			log.Printf("[%v] Error updating %v %v %v\n", calendarType, eventID, event.Summary, err)
		} else {
			log.Printf("[%v] Updated %v %v\n", calendarType, newEvent.Id, newEvent.Summary)

			if sessionID != "" {
				// update event id to app data
				saveNewEventID(appData, calendarType, sessionID, newEvent.Id)
			} else {
				log.Printf("[%v] Couldn't find session ID for given event [2]\n", calendarType)
			}
		}
	}

}

func findSessionIDForEvent(sessionsEventsMap map[string]*calendar.Event, event *calendar.Event) string {
	sessionID := ""
	for s, ev := range sessionsEventsMap {
		if ev == event {
			sessionID = s
			break
		}
	}
	if sessionID == "" {
		log.Println("Couldn't find session ID for given event", event.Summary)
	}
	return sessionID
}
func main() {

	// get cached app data
	appData := &AppData{}
	b, err := ioutil.ReadFile(CalendarDataFilename)
	err = json.Unmarshal(b, &appData)
	if err != nil {
		appData = &AppData{}
	}
	if appData.TrackCalendarIDs == nil || len(appData.TrackCalendarIDs) == 0 {
		appData.TrackCalendarIDs = map[string]string{}
	}
	if appData.LocationCalendarIDs == nil || len(appData.LocationCalendarIDs) == 0 {
		appData.LocationCalendarIDs = map[string]string{}
	}
	if appData.MasterCalendarSessionIDs == nil || len(appData.MasterCalendarSessionIDs) == 0 {
		appData.MasterCalendarSessionIDs = map[string]string{}
	}
	if appData.TrackCalendarSessionIDs == nil || len(appData.TrackCalendarSessionIDs) == 0 {
		appData.TrackCalendarSessionIDs = map[string]string{}
	}
	if appData.LocationCalendarSessionIDs == nil || len(appData.LocationCalendarSessionIDs) == 0 {
		appData.LocationCalendarSessionIDs = map[string]string{}
	}

	// get FOSSASIA 2016 sessions data (JSON)
	r, err := http.Get(SessionsJSONURL)
	defer r.Body.Close()
	if err != nil {
		panic(err)
	}

	// read and marshal data
	sessionsData := FOSSAsiaEvent{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, &sessionsData)

	// read calendar service key JWT config
	b, err = ioutil.ReadFile(ServiceKeyFilename)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// get calendar client
	ctx := context.Background()
	client := config.Client(ctx)
	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve calendar Client %v", err)
	}

	// use this to figure out if a session has been removed, so that we can clean up the calendar
	sessionsIDs := []string{}

	// create event for each session
	events := []*calendar.Event{}
	sessionsEventsMap := map[string]*calendar.Event{}
	for _, session := range sessionsData.Sessions {
		if session.Title == "" || session.StartTime == "" || session.EndTime == "" {
			continue
		}
		title := session.Title
		if session.Track.Name != "" {
			title = fmt.Sprintf("%v [%v]", session.Title, session.Track.Name)
		}
		description := fmt.Sprintf("Speaker(s): %v", Speakers(session.Speakers).String())
		if session.Type != "" {
			description = fmt.Sprintf("%v\n%v", session.Type, description)
		}
		if session.Description != "" {
			description = fmt.Sprintf("%v\n\n%v", session.Description, description)
		}

		// create event
		event := &calendar.Event{
			Summary:     title,
			Description: description,
			Location:    fmt.Sprintf("%v, Science Centre Singapore", session.Location),
			Start: &calendar.EventDateTime{
				DateTime: session.StartTime,
				TimeZone: "Asia/Singapore",
			},
			End: &calendar.EventDateTime{
				DateTime: session.EndTime,
				TimeZone: "Asia/Singapore",
			},
		}

		sessionsIDs = append(sessionsIDs, session.SessionID)

		events = append(events, event)
		sessionsEventsMap[session.SessionID] = event
	}

	// organize events by tracks
	eventTracksMap := map[int][]*calendar.Event{}
	trackIDs := []int{}
	tracksMap := map[int]Track{}
	for _, session := range sessionsData.Sessions {

		// collect track ids
		hasTrackID := false
		for _, trackID := range trackIDs {
			if trackID == session.Track.ID {
				hasTrackID = true
				break
			}
		}
		if !hasTrackID {
			trackIDs = append(trackIDs, session.Track.ID)
			tracksMap[session.Track.ID] = session.Track
		}

		if _, ok := eventTracksMap[session.Track.ID]; !ok {
			eventTracksMap[session.Track.ID] = []*calendar.Event{}
		}
		if event, ok := sessionsEventsMap[session.SessionID]; ok {
			eventTracksMap[session.Track.ID] = append(eventTracksMap[session.Track.ID], event)
		}
	}

	// organize sessions by location
	eventsLocationsMap := map[string][]*calendar.Event{}
	locations := []string{}
	for _, session := range sessionsData.Sessions {

		// scrub `location` value
		location := session.Location
		if location == "" {
			location = DefaultLocation
		}
		location, ok := ScrubbedLocation[location]
		if !ok {
			location = session.Location
		}

		// collect locations
		hasLocation := false
		for _, loc := range locations {
			if loc == location {
				hasLocation = true
				break
			}
		}
		if !hasLocation {
			locations = append(locations, location)
		}

		if _, ok := eventsLocationsMap[location]; !ok {
			eventsLocationsMap[location] = []*calendar.Event{}
		}
		if event, ok := sessionsEventsMap[session.SessionID]; ok {
			eventsLocationsMap[location] = append(eventsLocationsMap[location], event)
		}
	}

	// track calendar ids
	trackCalendarIDsMap := map[int]string{}
	locationCalendarIDsMap := map[string]string{}

	// create a master "ALL" calendar. why? because.
	masterCalendarID := appData.MasterCalendarID
	if masterCalendarID == "" {

		masterCalendarID, err = createCalendar(
			srv,
			"FOSSASIA 2016 - ALL",
			"FOSSASIA 2016 Schedule\nSource available at https://github.com/sogko/fossasia-2016-google-calendar",
		)

		appData.MasterCalendarID = masterCalendarID
	}

	// removed newly-deleted events from master calendar
	removeEntriesForDeletedSessionsFromCalendar(srv, appData.MasterCalendarSessionIDs, masterCalendarID, sessionsIDs)

	// for each track, create a calendar and add its events
	// at the same time, add events to "ALL" calendar
	for trackID, events := range eventTracksMap {

		// create calendar for track
		track, _ := tracksMap[trackID]
		trackIDStr := fmt.Sprintf("%v", trackID)
		calendarID, ok := appData.TrackCalendarIDs[trackIDStr]
		if calendarID == "" || !ok {

			calendarID, err = createCalendar(
				srv,
				fmt.Sprintf("FA16 - %v", track.Name),
				fmt.Sprintf("FOSSASIA 2016 Schedule - %v\nSource available at https://github.com/sogko/fossasia-2016-google-calendar", track.Name),
			)

			appData.TrackCalendarIDs[trackIDStr] = calendarID

		}

		// removed newly-deleted events from track calendar
		removeEntriesForDeletedSessionsFromCalendar(srv, appData.TrackCalendarSessionIDs, calendarID, sessionsIDs)

		// store created calendar ids
		trackCalendarIDsMap[trackID] = calendarID

		// add events into google calendar
		log.Printf("Inserting %v session entries for track %v\n", len(events), trackID)
		for _, event := range events {

			// find session id for given event
			sessionID := findSessionIDForEvent(sessionsEventsMap, event)

			// add or update event on track calendar
			insertOrUpdateEventForSession(srv, appData, "track", calendarID, sessionID, event)

			// add or update event on master calendar
			insertOrUpdateEventForSession(srv, appData, "master", calendarID, sessionID, event)

		}
	}

	// for each location, create a calendar and add its events
	for location, events := range eventsLocationsMap {

		// create calendar for location
		calendarID, ok := appData.LocationCalendarIDs[location]
		if calendarID == "" || !ok {
			calendarID, err = createCalendar(
				srv,
				fmt.Sprintf("FA16 @ %v", location),
				fmt.Sprintf("FOSSASIA 2016 Schedule at %v\nSource available at https://github.com/sogko/fossasia-2016-google-calendar", location),
			)

			appData.LocationCalendarIDs[location] = calendarID

		}

		// removed newly-deleted events from location calendar
		removeEntriesForDeletedSessionsFromCalendar(srv, appData.LocationCalendarSessionIDs, calendarID, sessionsIDs)

		// store created calendar ids
		locationCalendarIDsMap[location] = calendarID

		// add events into google calendar
		log.Printf("Inserting %v session entries for location %v\n", len(events), location)
		for _, event := range events {

			// find session id for given event
			sessionID := findSessionIDForEvent(sessionsEventsMap, event)

			// add or update event on location calendar
			insertOrUpdateEventForSession(srv, appData, "location", calendarID, sessionID, event)
		}
	}

	// print URL to add calendars
	fmt.Println("\n\nView MASTER calendar at: ", appData.GetMasterCalendarURL())
	fmt.Println("\n\nView TRACK calendar at: ", appData.GetTrackCalendarURL())
	fmt.Println("\n\nView LOCATION calendar at: ", appData.GetLocationCalendarURL())

	// store calendar data into cache
	buf := make([]byte, 0)
	out := bytes.NewBuffer(buf)
	j, err := json.MarshalIndent(&appData, "", "  ")
	if err != nil {
		log.Printf("Error marshalling calendar ids data%v\n", err)
	}
	ioutil.WriteFile(CalendarDataFilename, j, os.ModePerm)
	_, err = out.Write(j)
	if err != nil {
		log.Printf("Error writing calendar ids data%v\n", err)
	}
	if err == nil {
		fmt.Println("\nWrote calendar ids to: ", CalendarDataFilename)
	}

	log.Println("Done!")

}
