// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import (
	"encoding/json"
)

func encodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func decodeJSON(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}
