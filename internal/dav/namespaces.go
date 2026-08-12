// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import "encoding/xml"

const (
	NamespaceDAV     = "DAV:"
	NamespaceCalDAV  = "urn:ietf:params:xml:ns:caldav"
	NamespaceCardDAV = "urn:ietf:params:xml:ns:carddav"
	NamespaceApple   = "http://apple.com/ns/ical/"
	NamespaceCS      = "http://calendarserver.org/ns/"
)

var (
	ResourceTypeName                  = xml.Name{Space: NamespaceDAV, Local: "resourcetype"}
	DisplayNameName                   = xml.Name{Space: NamespaceDAV, Local: "displayname"}
	CurrentUserPrincipalName          = xml.Name{Space: NamespaceDAV, Local: "current-user-principal"}
	CurrentUserPrivilegeSetName       = xml.Name{Space: NamespaceDAV, Local: "current-user-privilege-set"}
	CollectionName                    = xml.Name{Space: NamespaceDAV, Local: "collection"}
	PrincipalName                     = xml.Name{Space: NamespaceDAV, Local: "principal"}
	CalendarName                      = xml.Name{Space: NamespaceCalDAV, Local: "calendar"}
	AddressBookName                   = xml.Name{Space: NamespaceCardDAV, Local: "addressbook"}
	CalendarHomeSetName               = xml.Name{Space: NamespaceCalDAV, Local: "calendar-home-set"}
	AddressBookHomeSetName            = xml.Name{Space: NamespaceCardDAV, Local: "addressbook-home-set"}
	CalendarColorName                 = xml.Name{Space: NamespaceApple, Local: "calendar-color"}
	SupportedCalendarComponentSetName = xml.Name{Space: NamespaceCalDAV, Local: "supported-calendar-component-set"}
	GetCTagName                       = xml.Name{Space: NamespaceCS, Local: "getctag"}
	GetETagName                       = xml.Name{Space: NamespaceDAV, Local: "getetag"}
	GetContentTypeName                = xml.Name{Space: NamespaceDAV, Local: "getcontenttype"}
	AddressDataName                   = xml.Name{Space: NamespaceCardDAV, Local: "address-data"}
	CalendarDataName                  = xml.Name{Space: NamespaceCalDAV, Local: "calendar-data"}
)
