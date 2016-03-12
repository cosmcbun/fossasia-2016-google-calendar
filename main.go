package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"bytes"
	"github.com/kr/pretty"
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

type Speaker struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Track struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
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
		res = append(res, speaker.Name)
	}
	return strings.Join(res, ", ")
}

type FOSSAsiaEvent struct {
	Sessions []*SessionEntry `json:"sessions"`
}

type FOSSAsiaCalendarIDs struct {
	MasterCalendarID string            `json:"master_calendar_id"`
	TrackCalendarIDs map[string]string `json:"track_calendar_ids"`
	URL              string            `json:"url"`
}

func main() {

	// get cached data
	calendarData := FOSSAsiaCalendarIDs{}
	b, err := ioutil.ReadFile(CalendarDataFilename)
	err = json.Unmarshal(b, &calendarData)
	if err != nil {
		calendarData = FOSSAsiaCalendarIDs{
			TrackCalendarIDs: map[string]string{},
		}
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

	// organize sessions by tracks
	sessionsTracksMap := map[int][]*SessionEntry{}
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

		if _, ok := sessionsTracksMap[session.Track.ID]; !ok {
			sessionsTracksMap[session.Track.ID] = []*SessionEntry{}
		}
		sessionsTracksMap[session.Track.ID] = append(sessionsTracksMap[session.Track.ID], session)
	}

	// read service key JWT config
	b, err = ioutil.ReadFile(ServiceKeyFilename)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// get calendar Client
	ctx := context.Background()
	client := config.Client(ctx)
	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve calendar Client %v", err)
	}

	trackCalendarIDsMap := map[int]string{}
	calendarIDs := []string{}

	// create a master "ALL" calendar. why? because.
	masterCalendarID := calendarData.MasterCalendarID
	if masterCalendarID == "" {
		calendarSummary := fmt.Sprintf("FOSSASIA 2016 - ALL")
		newCal, err := srv.Calendars.Insert(&calendar.Calendar{
			Description: "FOSSASIA 2016 Schedule\nSource available at https://github.com/sogko/fossasia-2016-google-calendar",
			Summary:     calendarSummary,
			TimeZone:    "Asia/Singapore",
			Location:    "Singapore",
		}).Do()
		if err != nil {
			log.Fatalf("Failed to create master calendar %v", err)
			return
		}
		masterCalendarID = newCal.Id
		calendarData.MasterCalendarID = masterCalendarID

		// create ACL for master calendar
		publicACLrule := &calendar.AclRule{
			Scope: &calendar.AclRuleScope{
				Type:  "default",
				Value: "",
			},
			Role: "reader",
		}
		publicACLrule, err = srv.Acl.Insert(masterCalendarID, publicACLrule).Do()
		if err != nil {
			log.Fatalf("Failed to set calendar ACL %v", err)
		}

	}
	// clear all existing events
	events, _ := srv.Events.List(masterCalendarID).Do()
	if events != nil {
		for _, event := range events.Items {
			log.Println("Deleting ", event.Id, event.Summary)
			srv.Events.Delete(masterCalendarID, event.Id).Do()
		}
	}
	calendarIDs = append(calendarIDs, masterCalendarID)

	// for each track, create a calendar and add its session entries
	// at the same time, add events to "ALL" calendar
	for trackID, sessions := range sessionsTracksMap {

		// create calendar for track
		track, _ := tracksMap[trackID]
		trackIDStr := fmt.Sprintf("%v", trackID)
		calendarID, ok := calendarData.TrackCalendarIDs[trackIDStr]
		if calendarID == "" || !ok {
			calendarSummary := fmt.Sprintf("FA16 - %v", track.Name)
			newCal, err := srv.Calendars.Insert(&calendar.Calendar{
				Description: fmt.Sprintf("FOSSASIA 2016 Schedule - %v\nSource available at https://github.com/sogko/fossasia-2016-google-calendar", track.Name),
				Summary:     calendarSummary,
				TimeZone:    "Asia/Singapore",
				Location:    "Singapore",
			}).Do()
			if err != nil {
				log.Fatalf("Failed to create calendar %v", err)
				return
			}
			calendarID = newCal.Id
			calendarData.TrackCalendarIDs[trackIDStr] = calendarID

			// create ACL for newly created calendar
			publicACLrule := &calendar.AclRule{
				Scope: &calendar.AclRuleScope{
					Type:  "default",
					Value: "",
				},
				Role: "reader",
			}
			publicACLrule, err = srv.Acl.Insert(calendarID, publicACLrule).Do()
			if err != nil {
				log.Fatalf("Failed to set calendar ACL %v", err)
			}

		}
		// clear all existing events
		events, _ := srv.Events.List(calendarID).Do()
		if events != nil {
			for _, event := range events.Items {
				log.Println("Deleting ", event.Id, event.Summary)
				srv.Events.Delete(calendarID, event.Id).Do()
			}
		}

		// store created calendar ids
		trackCalendarIDsMap[trackID] = calendarID
		calendarIDs = append(calendarIDs, calendarID)


		// add session entries into google calendar
		log.Printf("Inserting %v session entries for track %v\n", len(sessions), trackID)
		for _, session := range sessions {
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

			// add to track calendar
			event, err := srv.Events.Insert(calendarID, &calendar.Event{
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
			}).Do()
			if err != nil {
				log.Fatalf("Error inserting event\n", err)
			} else {
				log.Printf("Inserted %v %v\n", event.Id, event.Summary)
			}

			// add to master calendar
			event, err = srv.Events.Insert(masterCalendarID, &calendar.Event{
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
			}).Do()
			if err != nil {
				log.Fatalf("[master] Error inserting %v %v %v\n", event.Id, event.Summary, err)
			}
		}
	}

	pretty.Println("C", calendarData)

	// print URL to add calendars
	url := []string{GoogleCalendarURLBase, "?"}
	for _, calendarID := range calendarIDs {
		url = append(url, fmt.Sprintf("cid=%v&", calendarID))
	}
	calendarData.URL = strings.Join(url, "")
	fmt.Println("\n\nView calendars at: ", calendarData.URL)

	// store calendar data into cache
	buf := make([]byte, 0)
	out := bytes.NewBuffer(buf)
	j, err := json.MarshalIndent(&calendarData, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling calendar ids data%v\n", err)
	}
	ioutil.WriteFile(CalendarDataFilename, j, os.ModePerm)
	_, err = out.Write(j)
	if err != nil {
		log.Fatalf("Error writing calendar ids data%v\n", err)
	}
	if err == nil {
		fmt.Println("\nWrote calendar ids to: ", CalendarDataFilename)
	}

	log.Println("Done!")

}
