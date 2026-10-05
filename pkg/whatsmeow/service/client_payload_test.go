package whatsmeow_service

import (
	"sync"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

func TestInitializeWhatsAppVersionAppliesHandshakeOnce(t *testing.T) {
	original := store.GetWAVersion()
	t.Cleanup(func() { store.SetWAVersion(original); whatsAppVersionOnce = sync.Once{} })
	cfg := &config.Config{WhatsappVersionMajor: 2, WhatsappVersionMinor: 3000, WhatsappVersionPatch: 1049294121}
	initializeWhatsAppVersion(cfg)
	if store.GetWAVersion() != (store.WAVersionContainer{2, 3000, 1049294121}) {
		t.Fatal("configured version was not applied to the handshake")
	}
	cfg.WhatsappVersionPatch++
	initializeWhatsAppVersion(cfg)
	if store.GetWAVersion()[2] != 1049294121 {
		t.Fatal("reconnect rewrote process-wide handshake data")
	}
}

func TestClientPayloadKeepsInstancePropertiesIsolated(t *testing.T) {
	s, dir := testSQLiteStore(t)
	container, err := s.get(nil, &config.Config{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	original := proto.Clone(store.DeviceProps)
	for _, name := range []string{"First instance", "Second instance"} {
		client := whatsmeow.NewClient(container.NewDevice(), nil)
		configureClientPayload(client, name)
		payload := client.GetClientPayload()
		props := &waCompanionReg.DeviceProps{}
		if err := proto.Unmarshal(payload.GetDevicePairingData().GetDeviceProps(), props); err != nil {
			t.Fatal(err)
		}
		if props.GetOs() != name || !props.GetRequireFullSync() || props.GetPlatformType() != waCompanionReg.DeviceProps_CHROME {
			t.Fatalf("wrong pairing properties: %v", props)
		}
		v := store.GetWAVersion()
		if payload.GetUserAgent().GetAppVersion().GetTertiary() != v[2] {
			t.Fatal("handshake does not carry the library WhatsApp version")
		}
	}
	if !proto.Equal(original, store.DeviceProps) {
		t.Fatal("instance changed process-wide pairing properties")
	}
}
