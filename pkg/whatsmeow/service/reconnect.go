package whatsmeow_service

import "go.mau.fi/whatsmeow"

// A pairing socket must recover in place, keeping the device identity and QR
// ceremony. A full instance restart is reserved for an already paired device.
func shouldRestartClient(client *whatsmeow.Client) bool {
	return client != nil && client.Store != nil && client.Store.ID != nil
}
