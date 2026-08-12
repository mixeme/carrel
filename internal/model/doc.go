// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model holds the domain objects that travel between the DAV providers
// and the interface (spec §8).
//
// The parsed payload of an Object is unexported. Callers read a display view
// and change an object by applying a Patch that names the properties it
// touches; there is no way to hand back a whole object assembled from form
// fields. That is deliberate: rebuilding a vCard from the fields a form happens
// to know about is how X- properties, categories and attachments written by
// other clients disappear without anyone noticing (§8).
package model
