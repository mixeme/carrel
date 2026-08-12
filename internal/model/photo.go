// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/emersion/go-vcard"
)

// InlinePhoto is the decoded bytes of an embedded PHOTO property.
type InlinePhoto struct {
	Bytes     []byte
	MediaType string
}

// ExtractPhoto returns the inline image bytes of PHOTO, if any. A URI photo
// returns ok=false — it is shown through the proxy instead (§11).
func (o *Object) ExtractPhoto() (InlinePhoto, bool, error) {
	if o == nil || o.raw == nil {
		return InlinePhoto{}, false, ErrNotVCard
	}
	values := o.Property(vcard.FieldPhoto)
	if len(values) == 0 {
		return InlinePhoto{}, false, nil
	}
	v := values[0]
	desc := describePhoto(values)
	if desc.URI != "" {
		return InlinePhoto{}, false, nil
	}
	raw, mediaType, err := decodePhotoText(v.Text, desc.MediaType)
	if err != nil {
		return InlinePhoto{}, false, err
	}
	return InlinePhoto{Bytes: raw, MediaType: mediaType}, true, nil
}

// PhotoValue builds a PHOTO property value for the object's vCard version (§11).
func PhotoValue(version string, jpeg []byte) Value {
	version = strings.TrimSpace(version)
	if version == "" {
		version = DefaultVCardVersion
	}
	encoded := base64.StdEncoding.EncodeToString(jpeg)
	if strings.HasPrefix(version, "4") {
		return Text("data:image/jpeg;base64," + encoded)
	}
	return Text(encoded).
		WithParam("ENCODING", "b").
		WithParam(vcard.ParamType, "JPEG")
}

func decodePhotoText(text, hintType string) ([]byte, string, error) {
	text = strings.TrimSpace(text)
	mediaType := hintType
	if rest, ok := cutPrefix(text, "data:"); ok {
		meta, data, found := strings.Cut(rest, ",")
		if !found {
			return nil, "", fmt.Errorf("model: malformed data URI PHOTO")
		}
		parts := strings.Split(meta, ";")
		if len(parts) > 0 && parts[0] != "" {
			mediaType = strings.ToLower(parts[0])
		}
		isBase64 := false
		for _, p := range parts[1:] {
			if strings.EqualFold(p, "base64") {
				isBase64 = true
			}
		}
		if !isBase64 {
			return nil, "", fmt.Errorf("model: PHOTO data URI is not base64")
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			raw, err = base64.RawStdEncoding.DecodeString(data)
		}
		if err != nil {
			return nil, "", fmt.Errorf("model: decode PHOTO: %w", err)
		}
		if mediaType == "" {
			mediaType = "image/jpeg"
		}
		return raw, mediaType, nil
	}
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil {
		return nil, "", fmt.Errorf("model: decode PHOTO: %w", err)
	}
	if mediaType == "" {
		mediaType = "image/jpeg"
	}
	return raw, mediaType, nil
}
