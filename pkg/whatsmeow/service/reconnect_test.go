package whatsmeow_service

import (
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func TestOnlyPairedDevicesCanRestartOnDisconnect(t *testing.T) {
	if shouldRestartClient(nil) || shouldRestartClient(&whatsmeow.Client{}) || shouldRestartClient(&whatsmeow.Client{Store: &store.Device{}}) {
		t.Fatal("disconnect during QR pairing must not discard the device identity")
	}
	jid := types.NewJID("15551234567", types.DefaultUserServer)
	if !shouldRestartClient(&whatsmeow.Client{Store: &store.Device{ID: &jid}}) {
		t.Fatal("paired device must retain automatic recovery")
	}
}
