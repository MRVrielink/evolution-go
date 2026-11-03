package whatsmeow_service

import (
	"context"
	"errors"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const communityDecryptTimeout = 5 * time.Second

var errInvalidCommunityAction = errors.New("invalid encrypted community action")

// Called after edit unwrapping and before JID normalization. The owning client's
// secret store needs the original wire identity. Failures retain the ciphertext.
func prepareIncomingCommunityAction(ctx context.Context, client *whatsmeow.Client, evt *events.Message) (bool, error) {
	if evt == nil || evt.Message == nil {
		return false, nil
	}
	reaction := evt.Message.GetEncReactionMessage()
	comment := evt.Message.GetEncCommentMessage()
	if reaction == nil && comment == nil {
		return false, nil
	}
	var iv []byte
	var target *waCommon.MessageKey
	if reaction != nil {
		iv, target = reaction.GetEncIV(), reaction.GetTargetMessageKey()
	} else {
		iv, target = comment.GetEncIV(), comment.GetTargetMessageKey()
	}
	if len(iv) != 12 || target.GetID() == "" {
		return true, errInvalidCommunityAction
	}
	if client == nil {
		return true, whatsmeow.ErrClientIsNil
	}
	if client.Store == nil || client.Store.MsgSecrets == nil {
		return true, errors.New("message secret store unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, communityDecryptTimeout)
	defer cancel()
	if reaction != nil {
		decoded, err := client.DecryptReaction(ctx, evt)
		if err != nil {
			return true, err
		}
		if decoded.GetKey().GetID() != target.GetID() {
			return true, errInvalidCommunityAction
		}
		message := proto.Clone(evt.Message).(*waE2E.Message)
		message.EncReactionMessage = nil
		message.ReactionMessage = decoded
		evt.Message = message
	} else {
		message, err := client.DecryptComment(ctx, evt)
		if err != nil {
			return true, err
		}
		if message == nil || proto.Size(message) == 0 {
			return true, errInvalidCommunityAction
		}
		decoded := *evt
		unwrapMessageForWebhook(&decoded, message)
		*evt = decoded
	}
	return false, nil
}
