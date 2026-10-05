package whatsmeow_service

import (
	"sync"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"github.com/gomessguii/logger"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

var whatsAppVersionOnce sync.Once

// SetWAVersion changes global handshake data. Initialize it before any client
// connects and leave it immutable during reconnects and concurrent startups.
func initializeWhatsAppVersion(cfg *config.Config) {
	whatsAppVersionOnce.Do(func() {
		v := clientVersion{cfg.WhatsappVersionMajor, cfg.WhatsappVersionMinor, cfg.WhatsappVersionPatch}
		if v.Major == 0 || v.Minor == 0 || v.Patch == 0 {
			webVersion, err := fetchWhatsAppWebVersion()
			if err != nil {
				logger.LogWarn("WhatsApp Web version lookup failed, keeping library version %s: %v", store.GetWAVersion(), err)
				return
			}
			v = *webVersion
		}
		store.SetWAVersion(store.WAVersionContainer{uint32(v.Major), uint32(v.Minor), uint32(v.Patch)})
	})
}

// Device properties belong to the instance. Mutating store.DeviceProps while
// other instances are pairing races their protobuf serialization.
func configureClientPayload(client *whatsmeow.Client, osName string) {
	if osName == "" {
		osName = utils.WhatsAppGetUserOS()
	}
	props := proto.Clone(store.DeviceProps).(*waCompanionReg.DeviceProps)
	props.Os = proto.String(osName)
	props.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()
	props.RequireFullSync = proto.Bool(true)
	version := store.GetWAVersion()
	props.Version = &waCompanionReg.DeviceProps_AppVersion{
		Primary: proto.Uint32(version[0]), Secondary: proto.Uint32(version[1]), Tertiary: proto.Uint32(version[2]),
	}
	deviceProps, _ := proto.Marshal(props)
	client.GetClientPayload = func() *waWa6.ClientPayload {
		payload := client.Store.GetClientPayload()
		if payload.DevicePairingData != nil {
			payload.DevicePairingData.DeviceProps = append([]byte(nil), deviceProps...)
		}
		return payload
	}
}
