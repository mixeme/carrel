// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import "encoding/xml"

const (
	NamespaceDAV   = "DAV:"
	NamespaceCalDAV  = "urn:ietf:params:xml:ns:caldav"
	NamespaceCardDAV = "urn:ietf:params:xml:ns:carddav"
	NamespaceApple   = "http://apple.com/ns/ical/"
	NamespaceCS      = "http://calendarserver.org/ns/"
)

var (
	ResourceTypeName            = xml.Name{NamespaceDAV, "resourcetype"}
	DisplayNameName             = xml.Name{NamespaceDAV, "displayname"}
	CurrentUserPrincipalName    = xml.Name{NamespaceDAV, "current-user-principal"}
	CurrentUserPrivilegeSetName = xml.Name{NamespaceDAV, "current-user-privilege-set"}
	CollectionName              = xml.Name{NamespaceDAV, "collection"}
	PrincipalName               = xml.Name{NamespaceDAV, "principal"}
	CalendarName                = xml.Name{NamespaceCalDAV, "calendar"}
	AddressBookName             = xml.Name{NamespaceCardDAV, "addressbook"}
	CalendarHomeSetName         = xml.Name{NamespaceCalDAV, "calendar-home-set"}
	AddressBookHomeSetName      = xml.Name{NamespaceCardDAV, "addressbook-home-set"}
	CalendarColorName           = xml.Name{NamespaceApple, "calendar-color"}
	SupportedCalendarComponentSetName = xml.Name{NamespaceCalDAV, "supported-calendar-component-set"}
	GetCTagName                 = xml.Name{NamespaceCS, "getctag"}
)
