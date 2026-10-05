package whatsmeow_service

import "go.mau.fi/whatsmeow/types/events"

func archiveWebhookData(evt *events.Archive) map[string]interface{} {
	return map[string]interface{}{
		"JID": evt.JID, "Timestamp": evt.Timestamp, "Action": evt.Action, "FromFullSync": evt.FromFullSync,
	}
}
