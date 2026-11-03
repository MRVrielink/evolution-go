package send_service

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStickerAnimatedWebPPassthrough(t *testing.T) {
	// A RIFF container with the VP8X animation flag must not be statically encoded.
	payload := make([]byte, 30)
	copy(payload, "RIFF")
	binary.LittleEndian.PutUint32(payload[4:8], uint32(len(payload)-8))
	copy(payload[8:], "WEBPVP8X")
	binary.LittleEndian.PutUint32(payload[16:20], 10)
	payload[20] = 0x02
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	got, err := stickerWebP(context.Background(), server.URL)
	if err != nil || !bytes.Equal(got, payload) || !webpIsAnimated(got) {
		t.Fatalf("animation changed: %x %v", got, err)
	}
	static := bytes.Clone(payload)
	static[20] = 0
	if webpIsAnimated(static) {
		t.Fatal("static sticker classified as animated")
	}
	binary.LittleEndian.PutUint32(static[4:8], 1000)
	if isWebP(static) {
		t.Fatal("truncated WebP accepted")
	}
}

func TestStickerFetchFailureAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := stickerWebP(context.Background(), server.URL); err == nil {
		t.Fatal("HTTP error accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stickerWebP(ctx, server.URL); err == nil {
		t.Fatal("canceled fetch accepted")
	}
}
