package instance_service

import (
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
)

type disconnectRepository struct {
	instance_repository.InstanceRepository
	id        string
	connected bool
	reason    string
}

func (r *disconnectRepository) UpdateConnected(id string, connected bool, reason string) error {
	r.id, r.connected, r.reason = id, connected, reason
	return nil
}

func TestDisconnectPreservesSubscriptions(t *testing.T) {
	repo := &disconnectRepository{}
	service := instances{instanceRepository: repo}
	instance := &instance_model.Instance{Id: "test-id", Events: "MESSAGE,CONNECTION,CHAT_PRESENCE", Connected: true, Webhook: "https://example.invalid/webhook"}
	if err := service.recordDisconnect(instance); err != nil {
		t.Fatal(err)
	}
	if instance.Events != "MESSAGE,CONNECTION,CHAT_PRESENCE" || instance.Webhook != "https://example.invalid/webhook" {
		t.Fatal("disconnect erased event delivery configuration")
	}
	if instance.Connected || repo.connected || repo.id != instance.Id || repo.reason != "Disconnected by API" {
		t.Fatalf("disconnect was not persisted: %+v", repo)
	}
}
