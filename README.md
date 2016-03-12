# fossasia-2016-google-calendar
A quick and dirty little thing that takes FOSSASIA 2016 schedule and creates Google Calendars out of it.

![Screenshot](http://i.imgur.com/Zrwzyvn.png)

## Why?
Because.

## How?
FOSSASIA has its schedule in JSON over here: https://raw.githubusercontent.com/fossasia/open-event-scraper/master/out/sessions.json.

Squirt out a quick one that uses it to import the events into Google Calendars.

There are over 300 events* scheduled over 3-days, it makes it hard to have it all in one big calendar.

I've separated different topics (they call it `tracks`) into separate calendars, so you can filter events based on your
topic of interest when deciding which talks to attend.

## Google Calendar URLS
Click the following links to add FOSSASIA 2016 schedule to your Google Calendar


[FOSS ASIA 2016 - ALL](https://calendar.google.com/calendar/render?cid=oqrj3a93g17r1pgckrr2sv4klc@group.calendar.google.com)
All events in one calendar

[FOSS ASIA 2016 - By Topics/Tracks](https://calendar.google.com/calendar/render?cid=gb9e6o5rhngojiooltgs73gbks@group.calendar.google.com&cid=dgdout91jqtgo5ir5krsghgq6g@group.calendar.google.com&cid=oeeack2tej4hepfnn5sl86ijv8@group.calendar.google.com&cid=efmcpobnjnflhg7p6amjjnucfo@group.calendar.google.com&cid=2u592kc3v676evfrchmmhtddhc@group.calendar.google.com&cid=2t6srepd45g1sp2igr9uhs1foc@group.calendar.google.com&cid=1sjun8vbpda14fhcdnb4ooku50@group.calendar.google.com&cid=ss8o2s4o1i71tbff7pvuhaf1ps@group.calendar.google.com&cid=o4lp7slj2k55rf9c5sfuavuegg@group.calendar.google.com&cid=0br88vtd48kk9rmeg7p02saueo@group.calendar.google.com&cid=9a5ddn4f5milnq5dmvu7i4pod0@group.calendar.google.com&cid=5jfq646ll31ovmnn5iad509c60@group.calendar.google.com&cid=so7meffbc6geit92c0veucn478@group.calendar.google.com&cid=bgichra2s39578j1s12cuovhkc@group.calendar.google.com&cid=llvl3lc03e0gggb29di6jf31k4@group.calendar.google.com&cid=5qg5cdi4qkr7n05eqgd9juhs6k@group.calendar.google.com&cid=mk37q05h3b4994boqbt534gmns@group.calendar.google.com)
Separate calendars for each topic / tracks


## Wanna to run it?

#### >= Go 1.5
You need >= Go 1.5 to run this sexy little chica.

#### service_key.json
Visit https://console.developers.google.com and create a Service Account key for Calendar product.
Replace the content of `service_key.json` with your private key and boomz.

#### data.json
This file caches previously created calendar IDs, so that we can update the calendar without pillaging it like a band of vikings. 

Note: if you use your own Service Account key, you won't be able to access the calendars stored in `data.json`, so clear it before you run it.

#### sessions.json
This file is take from [here](https://raw.githubusercontent.com/fossasia/open-event-scraper/master/out/sessions.json). 
This app itself re-downloads it every time it's run.


## Disclaimer
This code was cobbled up during a lazy and hot Saturday afternoon. It's one of those "it-works" kind of thing.


