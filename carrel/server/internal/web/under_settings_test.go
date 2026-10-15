package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Лс29: a sheet of settings or administration opened by its address stands
// over the page it was opened from, as its frame draws it (the owner's
// decision of 15.10.2026): the password over «Set up a new device»
// (s-plan-3) or Connections (s-profile-3), Disconnect over Connections, the
// export's choice over the list, the reset of a password over Users
// (s-admin-13).
func TestSettingsSheetsStandOverTheirPage(t *testing.T) {
	f := calendarFixture(t, nil)
	fromDevices := decode[view.UnlockSheet](t, f.h.do(get("/app/settings/unlock?back="+url.QueryEscape("/app/settings/devices"), f.sess)))
	if fromDevices.Under == nil || fromDevices.Under.Devices == nil {
		t.Errorf("the password opened from devices stands over %+v", fromDevices.Under)
	}
	alone := decode[view.UnlockSheet](t, f.h.do(get("/app/settings/unlock", f.sess)))
	if alone.Under == nil || alone.Under.Connections == nil {
		t.Errorf("the password opened from nowhere stands over %+v", alone.Under)
	}
	off := decode[view.ConnectionRemoveSheet](t, f.h.do(get("/app/settings/connections/disconnect?"+url.Values{"account": {f.id}}.Encode(), f.sess)))
	if off.Under == nil || off.Under.Connections == nil {
		t.Errorf("Disconnect stands over %+v", off.Under)
	}
	export := decode[view.ExportSheet](t, f.h.do(get("/app/calendar/export", f.sess)))
	if export.Under == nil || export.Under.Agenda == nil {
		t.Errorf("the export's choice stands over %+v", export.Under)
	}

	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")
	reset := decode[view.PasswordResetSheet](t, h.do(get("/admin/reset?target="+bobID, alice)))
	if reset.Under == nil || reset.Under.AdminUsers == nil {
		t.Errorf("the reset stands over %+v", reset.Under)
	}
}
