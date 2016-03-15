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

const DefaultCalendarID = "jtnlrhbvsrh8jvjdqku441uq74@group.calendar.google.com" // set it to blank to create a new calendar
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
	MasterCalendarID    string            `json:"master_calendar_id"`
	TrackCalendarIDs    map[string]string `json:"track_calendar_ids"`
	LocationCalendarIDs map[string]string `json:"location_calendar_ids"`
	MasterCalendarURL   string            `json:"master_calendar_url"`
	TrackCalendarURL    string            `json:"track_calendar_url"`
	LocationCalendarURL string            `json:"location_calendar_url"`
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
func clearCalendar(srv *calendar.Service, calendarID string) {
	events, err := srv.Events.List(calendarID).MaxResults(2500).SingleEvents(true).ShowDeleted(false).Do()
	if err != nil {
		log.Printf(`Error retrieving events for %v`, calendarID)
	}
	if events != nil {
		for _, event := range events.Items {
			log.Println("Deleting ", event.Id, event.Summary, calendarID)
			err := srv.Events.Delete(calendarID, event.Id).Do()
			if err != nil {
				log.Printf(`Error deleting event %v`, event.Id)
			}
		}
	}
}

func main() {

	// get cached app data
	appData := AppData{}
	b, err := ioutil.ReadFile(CalendarDataFilename)
	err = json.Unmarshal(b, &appData)
	if err != nil {
		appData = AppData{}
	}
	if appData.TrackCalendarIDs == nil || len(appData.TrackCalendarIDs) == 0 {
		appData.TrackCalendarIDs = map[string]string{}
	}
	if appData.LocationCalendarIDs == nil || len(appData.LocationCalendarIDs) == 0 {
		appData.LocationCalendarIDs = map[string]string{}
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

	// create events for each session
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
	masterCalendarID := calendarData.MasterCalendarID
	if masterCalendarID == "" {

		masterCalendarID, err = createCalendar(
			srv,
			"FOSSASIA 2016 - ALL",
			"FOSSASIA 2016 Schedule\nSource available at https://github.com/sogko/fossasia-2016-google-calendar",
		)

		calendarData.MasterCalendarID = masterCalendarID
	}
	// clear all existing events
	clearCalendar(srv, masterCalendarID)

	// for each track, create a calendar and add its events
	// at the same time, add events to "ALL" calendar
	for trackID, events := range eventTracksMap {

		// create calendar for track
		track, _ := tracksMap[trackID]
		trackIDStr := fmt.Sprintf("%v", trackID)
		calendarID, ok := calendarData.TrackCalendarIDs[trackIDStr]
		if calendarID == "" || !ok {

			calendarID, err = createCalendar(
				srv,
				fmt.Sprintf("FA16 - %v", track.Name),
				fmt.Sprintf("FOSSASIA 2016 Schedule - %v\nSource available at https://github.com/sogko/fossasia-2016-google-calendar", track.Name),
			)

			calendarData.TrackCalendarIDs[trackIDStr] = calendarID

		}
		// clear all existing events
		clearCalendar(srv, calendarID)

		// store created calendar ids
		trackCalendarIDsMap[trackID] = calendarID

		// add events into google calendar
		log.Printf("Inserting %v session entries for track %v\n", len(events), trackID)
		for _, event := range events {

			// add event to track calendar
			newEvent, err := srv.Events.Insert(calendarID, event).Do()
			if err != nil {
				log.Printf("Error inserting event\n", err)
			} else {
				log.Printf("Inserted %v %v\n", newEvent.Id, newEvent.Summary)
			}

			// add to master calendar
			event, err = srv.Events.Insert(masterCalendarID, event).Do()
			if err != nil {
				log.Printf("[master] Error inserting %v %v %v\n", event.Id, event.Summary, err)
			}
		}
	}

	// for each location, create a calendar and add its events
	for location, events := range eventsLocationsMap {

		// create calendar for location
		calendarID, ok := calendarData.LocationCalendarIDs[location]
		if calendarID == "" || !ok {
			calendarID, err = createCalendar(
				srv,
				fmt.Sprintf("FA16 @ %v", location),
				fmt.Sprintf("FOSSASIA 2016 Schedule at %v\nSource available at https://github.com/sogko/fossasia-2016-google-calendar", location),
			)

			calendarData.LocationCalendarIDs[location] = calendarID

		}
		// clear all existing events
		clearCalendar(srv, calendarID)

		// store created calendar ids
		locationCalendarIDsMap[location] = calendarID

		// add events into google calendar
		log.Printf("Inserting %v session entries for location %v\n", len(events), location)
		for _, event := range events {

			// add event to location calendar
			newEvent, err := srv.Events.Insert(calendarID, event).Do()
			if err != nil {
				log.Printf("Error inserting event\n", err)
			} else {
				log.Printf("Inserted %v %v\n", newEvent.Id, newEvent.Summary)
			}
		}
	}

	// print URL to add calendars
	fmt.Println("\n\nView MASTER calendar at: ", calendarData.GetMasterCalendarURL())
	fmt.Println("\n\nView TRACK calendar at: ", calendarData.GetTrackCalendarURL())
	fmt.Println("\n\nView LOCATION calendar at: ", calendarData.GetLocationCalendarURL())

	// store calendar data into cache
	buf := make([]byte, 0)
	out := bytes.NewBuffer(buf)
	j, err := json.MarshalIndent(&calendarData, "", "  ")
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
