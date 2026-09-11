package dav

import (
	"context"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

// CalReporter covers CalDAV REPORT operations kept outside Transport (S-5).
type CalReporter interface {
	QueryCalendar(ctx context.Context, calendar string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error)
	MultiGetCalendar(ctx context.Context, path string, multiGet *caldav.CalendarMultiGet) ([]caldav.CalendarObject, error)
	SyncCollection(ctx context.Context, path string, query *caldav.SyncQuery) (*caldav.SyncResponse, error)
}

// CardReporter covers CardDAV REPORT operations kept outside Transport (S-5).
type CardReporter interface {
	QueryAddressBook(ctx context.Context, addressBook string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error)
	MultiGetAddressBook(ctx context.Context, path string, multiGet *carddav.AddressBookMultiGet) ([]carddav.AddressObject, error)
	SyncCollection(ctx context.Context, path string, query *carddav.SyncQuery) (*carddav.SyncResponse, error)
}
