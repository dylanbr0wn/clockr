package google

import "github.com/dylanbr0wn/shiet/internal/service"

type EventForTest = event
type EventTimeForTest = eventTime
type PersonForTest = person
type AttendeeForTest = attendee

func MapEventForTest(calendarID int64, ev EventForTest) (service.IncomingEvent, error) {
	return mapEvent(calendarID, ev)
}
