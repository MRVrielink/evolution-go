package whatsmeow_service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	"google.golang.org/protobuf/proto"
)

func encryptedCommunityFixture(t *testing.T, isReaction bool) (*events.Message, *whatsmeow.Client) {
	t.Helper()
	info := editTestInfo()
	secret := bytes.Repeat([]byte{0x42}, 32)
	sender := info.Sender.ToNonAD().String()
	target := &waCommon.MessageKey{ID: proto.String(editTestTargetID), FromMe: proto.Bool(true)}
	var plain []byte
	var err error
	useCase := whatsmeow.EncSecretComment
	if isReaction {
		useCase = whatsmeow.EncSecretReaction
		plain, err = proto.Marshal(&waE2E.ReactionMessage{Key: target, Text: proto.String("\U0001f44d")})
	} else {
		plain, err = proto.Marshal(&waE2E.Message{Conversation: proto.String("community reply")})
	}
	if err != nil {
		t.Fatal(err)
	}
	key := hkdfutil.SHA256(secret, nil, []byte(editTestTargetID+sender+sender+string(useCase)), 32)
	iv := bytes.Repeat([]byte{0x24}, 12)
	ciphertext, err := gcmutil.Encrypt(key, iv, plain, nil)
	if err != nil {
		t.Fatal(err)
	}
	message := &waE2E.Message{}
	if isReaction {
		message.EncReactionMessage = &waE2E.EncReactionMessage{EncIV: iv, EncPayload: ciphertext, TargetMessageKey: target}
	} else {
		message.EncCommentMessage = &waE2E.EncCommentMessage{EncIV: iv, EncPayload: ciphertext, TargetMessageKey: target}
	}
	secrets := &editTestSecretStore{get: func(ctx context.Context, chat, originalSender types.JID, id types.MessageID) ([]byte, types.JID, error) {
		if chat != info.Chat || originalSender != info.Sender || id != editTestTargetID {
			t.Error("community secret lookup changed wire identity")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > communityDecryptTimeout {
			t.Error("community secret lookup is not bounded")
		}
		return secret, info.Sender, nil
	}}
	return &events.Message{Info: info, Message: message, RawMessage: message}, &whatsmeow.Client{Store: &store.Device{MsgSecrets: secrets}}
}

func TestCommunityActionsDecryptAndReachWebhook(t *testing.T) {
	for _, isReaction := range []bool{true, false} {
		event, owner := encryptedCommunityFixture(t, isReaction)
		wire := proto.Clone(event.Message)
		handler, capture := editHandlerFixture(t, owner)
		handler.myEventHandler(event)
		data := capturedEditWebhook(t, capture)
		message := data["Message"].(map[string]interface{})
		if _, failed := data["decryptFailed"]; failed {
			t.Fatal("valid community action failed to decrypt", data)
		}
		if isReaction {
			if message["reactionMessage"] == nil || message["encReactionMessage"] != nil {
				t.Fatal("reaction did not reach webhook in plaintext", message)
			}
		} else if message["conversation"] != "community reply" || message["encCommentMessage"] != nil {
			t.Fatal("comment did not reach webhook in plaintext", message)
		}
		if !proto.Equal(wire, event.Message) || !proto.Equal(wire, event.RawMessage) {
			t.Fatal("handler changed the caller's encrypted event")
		}
	}
}

func TestCommunityActionsRejectMalformedNonceAndRetainCiphertext(t *testing.T) {
	for _, isReaction := range []bool{true, false} {
		for _, malformed := range []bool{true, false} {
			event, owner := encryptedCommunityFixture(t, isReaction)
			if malformed {
				if isReaction {
					event.Message.EncReactionMessage.EncIV = []byte{1}
				} else {
					event.Message.EncCommentMessage.EncIV = []byte{1}
				}
			} else {
				owner.Store.MsgSecrets = nil
			}
			wire := proto.Clone(event.Message)
			failed, err := prepareIncomingCommunityAction(context.Background(), owner, event)
			if !failed || err == nil || !proto.Equal(wire, event.Message) {
				t.Fatal("invalid action did not preserve its envelope")
			}
		}
	}
}
