// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package installcheck

import (
	"encoding/json"
	"io"
)

func jsonDecode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
