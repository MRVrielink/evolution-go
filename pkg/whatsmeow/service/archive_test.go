package whatsmeow_service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type archiveCaptureService struct {
	WhatsmeowService
	payload chan []byte
}

func (s *archiveCaptureService) CallWebhook(_ *instance_model.Instance, _ string, payload []byte) {
	s.payload <- payload
}

func TestArchiveEventReachesWebhook(t *testing.T) {
	cfg := &config.Config{LogDirectory: t.TempDir(), LogType: "console"}
	service := &archiveCaptureService{payload: make(chan []byte, 1)}
	client := &MyClient{
		service: service, config: cfg, userID: "archive-test", token: "test-token",
		Instance: &instance_model.Instance{Name: "Test"}, loggerWrapper: logger_wrapper.NewLoggerManager(cfg),
	}
	t.Cleanup(func() { _ = client.loggerWrapper.GetLogger(client.userID).Close() })
	client.myEventHandler(&events.Archive{
		JID: types.NewJID("15551234567", types.DefaultUserServer), Timestamp: time.Unix(1700000000, 0),
		Action: &waSyncAction.ArchiveChatAction{Archived: proto.Bool(true)}, FromFullSync: true,
	})
	select {
	case raw := <-service.payload:
		var payload struct {
			Event string `json:"event"`
			Data  struct {
				JID          string
				FromFullSync bool
				Action       struct {
					Archived bool `json:"archived"`
				}
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Event != "Archive" || payload.Data.JID != "15551234567@s.whatsapp.net" || !payload.Data.FromFullSync || !payload.Data.Action.Archived {
			t.Fatalf("archive webhook lost its original contract: %s", raw)
		}
	case <-time.After(time.Second):
		t.Fatal("archive event never reached webhook")
	}
}
