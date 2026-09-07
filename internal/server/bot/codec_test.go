package bot

import (
	"strings"
	"testing"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
	"github.com/go-sphere/telegram-bot/telegram"
	"github.com/go-telegram/bot/models"
)

func TestEncodeUpdateCountResponse(t *testing.T) {
	message, err := (MenuServiceBotCodec{}).EncodeUpdateCountResponse(t.Context(), &botv1.UpdateCountResponse{Value: 4})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	// Counter rows (-1/+1, Reset) plus the shared home row.
	if message.Text != "Counter: 4" || len(message.Button) != 3 || len(message.Button[0]) != 2 {
		t.Fatalf("unexpected message: %+v", message)
	}
	// The "-1" button must carry the count callback route plus the current value
	// and offset as the typed payload.
	if !strings.HasPrefix(message.Button[0][0].CallbackData, "count:") {
		t.Fatalf("callback data does not contain the route: %q", message.Button[0][0].CallbackData)
	}
	if !strings.Contains(message.Button[0][0].CallbackData, "-1") {
		t.Fatalf("callback data does not contain the offset payload: %q", message.Button[0][0].CallbackData)
	}
	if !strings.HasPrefix(message.Button[2][0].CallbackData, "nav_home:") {
		t.Fatalf("home button must carry the nav_home route: %q", message.Button[2][0].CallbackData)
	}
}

func TestEncodeCounterSingleButtonAtZero(t *testing.T) {
	// The keyboard changes with the value: a zero counter renders a single
	// "+1" button plus the home row, and no media (text-only menu).
	message, err := (MenuServiceBotCodec{}).EncodeCounterResponse(t.Context(), &botv1.CounterResponse{Value: 0})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Text != "Counter: 0" || len(message.Button) != 2 || len(message.Button[0]) != 1 {
		t.Fatalf("unexpected message: %+v", message)
	}
	if message.Media != nil {
		t.Fatalf("counter must be text-only, got %T", message.Media)
	}
}

func TestEncodeHomeResponse(t *testing.T) {
	// The home screen always carries a banner photo and a navigation grid with
	// one button per demo, each bound to the matching callback route.
	message, err := (MenuServiceBotCodec{}).EncodeHomeResponse(t.Context(), &botv1.HomeResponse{})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media == nil {
		t.Fatal("home screen must carry the banner media")
	}
	if len(message.Button) != 3 || len(message.Button[0]) != 2 || len(message.Button[1]) != 2 || len(message.Button[2]) != 1 {
		t.Fatalf("unexpected home keyboard: %+v", message.Button)
	}
	for _, want := range []string{"nav_counter:", "catalog:", "nav_menu:", "nav_photo:", "nav_help:"} {
		found := false
		for _, row := range message.Button {
			for _, button := range row {
				if strings.HasPrefix(button.CallbackData, want) {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("home keyboard missing button with route %q: %+v", want, message.Button)
		}
	}
}

func TestEncodeCatalogResponseFirstPage(t *testing.T) {
	// Page 1 of 3: three item rows, a pagination row without "Prev", and the
	// home row. Item buttons carry the catalog_item route with id + page.
	message, err := (MenuServiceBotCodec{}).EncodeCatalogResponse(t.Context(), &botv1.CatalogResponse{
		Page:       0,
		TotalPages: 3,
		Items: []*botv1.CatalogEntry{
			{Id: 1, Name: "one"},
			{Id: 2, Name: "two"},
			{Id: 3, Name: "three"},
		},
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media != nil {
		t.Fatalf("catalog must be text-only, got %T", message.Media)
	}
	if len(message.Button) != 5 || len(message.Button[0]) != 1 {
		t.Fatalf("unexpected catalog keyboard: %+v", message.Button)
	}
	pageRow := message.Button[3]
	if len(pageRow) != 2 || pageRow[0].Text != "1/3" || pageRow[1].Text != "Next ⏩" {
		t.Fatalf("unexpected pagination row: %+v", pageRow)
	}
	if !strings.HasPrefix(message.Button[0][0].CallbackData, "catalog_item:") {
		t.Fatalf("item button must carry the catalog_item route: %q", message.Button[0][0].CallbackData)
	}
	if !strings.HasPrefix(message.Button[4][0].CallbackData, "nav_home:") {
		t.Fatalf("home button must carry the nav_home route: %q", message.Button[4][0].CallbackData)
	}
}

func TestEncodeCatalogResponseLastPage(t *testing.T) {
	// On the last page the pagination row hides "Next" and keeps "Prev".
	message, err := (MenuServiceBotCodec{}).EncodeCatalogResponse(t.Context(), &botv1.CatalogResponse{
		Page:       2,
		TotalPages: 3,
		Items: []*botv1.CatalogEntry{
			{Id: 7, Name: "seven"},
		},
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	pageRow := message.Button[1]
	if len(pageRow) != 2 || pageRow[0].Text != "⏪ Prev" || pageRow[1].Text != "3/3" {
		t.Fatalf("unexpected pagination row: %+v", pageRow)
	}
}

func TestEncodeCatalogItemResponse(t *testing.T) {
	// The detail page has a quantity stepper, a back button carrying the
	// originating page on the "catalog" route, and the home row.
	message, err := (MenuServiceBotCodec{}).EncodeCatalogItemResponse(t.Context(), &botv1.CatalogItemResponse{
		Id:          2,
		Page:        1,
		Count:       3,
		Name:        "🪙 Coin bundle",
		Description: "A bundle of in-app coins.",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if !strings.Contains(message.Text, "Quantity: 3") {
		t.Fatalf("unexpected detail text: %q", message.Text)
	}
	if len(message.Button) != 3 || len(message.Button[0]) != 3 {
		t.Fatalf("unexpected detail keyboard: %+v", message.Button)
	}
	back := message.Button[1][0]
	if back.Text != "↩ Back" || !strings.HasPrefix(back.CallbackData, "catalog:") {
		t.Fatalf("back button must carry the catalog route: %+v", back)
	}
}

func TestEncodeShowPhotoResponse(t *testing.T) {
	message, err := (MenuServiceBotCodec{}).EncodeShowPhotoResponse(t.Context(), &botv1.ShowPhotoResponse{
		PhotoUrl: "https://example.com/a.jpg",
		Caption:  "cap",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media == nil {
		t.Fatal("photo message must carry media")
	}
	if _, ok := message.Media.(*models.InputFileString); !ok {
		t.Fatalf("expected a URL input file, got %T", message.Media)
	}
	// Swap/caption row plus the shared home row.
	if len(message.Button) != 2 || len(message.Button[0]) != 2 {
		t.Fatalf("unexpected keyboard: %+v", message.Button)
	}
	if !strings.HasPrefix(message.Button[0][0].CallbackData, "photo:") {
		t.Fatalf("swap button must carry the photo route: %q", message.Button[0][0].CallbackData)
	}
	if !strings.HasPrefix(message.Button[1][0].CallbackData, "nav_home:") {
		t.Fatalf("home button must carry the nav_home route: %q", message.Button[1][0].CallbackData)
	}
}

func TestEncodeEditPhotoResponseCaptionOnly(t *testing.T) {
	// A caption-only edit must not carry media, so the sender edits the caption
	// of the existing message instead of replacing its photo.
	message, err := (MenuServiceBotCodec{}).EncodeEditPhotoResponse(t.Context(), &botv1.EditPhotoResponse{Caption: "new cap"})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media != nil {
		t.Fatalf("caption-only edit must not carry media, got %T", message.Media)
	}
	if message.Text != "new cap" {
		t.Fatalf("caption = %q, want %q", message.Text, "new cap")
	}
}

func TestEncodeShowMenuResponseTextOnly(t *testing.T) {
	// A /menu card starts without an image: no media, one row of item buttons
	// (2 by default), the fixed control row of 4 buttons, and the home row.
	message, err := (MenuServiceBotCodec{}).EncodeShowMenuResponse(t.Context(), &botv1.ShowMenuResponse{
		Size:    2,
		Caption: "card",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media != nil {
		t.Fatalf("text-only card must not carry media, got %T", message.Media)
	}
	if len(message.Button) != 3 || len(message.Button[0]) != 2 || len(message.Button[1]) != 4 || len(message.Button[2]) != 1 {
		t.Fatalf("unexpected keyboard: %+v", message.Button)
	}
	if !strings.HasPrefix(message.Button[0][0].CallbackData, "menu:") {
		t.Fatalf("item button must carry the menu route: %q", message.Button[0][0].CallbackData)
	}
	if !strings.HasPrefix(message.Button[2][0].CallbackData, "nav_home:") {
		t.Fatalf("home button must carry the nav_home route: %q", message.Button[2][0].CallbackData)
	}
}

func TestEncodeEditMenuResponseWithImage(t *testing.T) {
	// The "image" action attaches the photo; the sender upgrades a text card
	// with EditMessageMedia or replaces the photo on a media card.
	message, err := (MenuServiceBotCodec{}).EncodeEditMenuResponse(t.Context(), &botv1.EditMenuResponse{
		Size:     3,
		PhotoUrl: "https://example.com/card.jpg",
		Caption:  "with image",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media == nil {
		t.Fatal("image card must carry media")
	}
	if _, ok := message.Media.(*models.InputFileString); !ok {
		t.Fatalf("expected a URL input file, got %T", message.Media)
	}
	// Item row (3), control row (4), home row (1).
	if len(message.Button) != 3 || len(message.Button[0]) != 3 {
		t.Fatalf("unexpected keyboard: %+v", message.Button)
	}
}

func TestEncodeEditMenuResponseKeepsSizeOnPick(t *testing.T) {
	// Picking an item renders a text-only caption (no media, so the sender
	// edits text or caption depending on the original card) and keeps the
	// keyboard size carried in the callback payload.
	message, err := (MenuServiceBotCodec{}).EncodeEditMenuResponse(t.Context(), &botv1.EditMenuResponse{
		Size:    2,
		Caption: "You picked 🍎 Apple (keyboard size 2).",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media != nil {
		t.Fatalf("pick must not carry media, got %T", message.Media)
	}
	if len(message.Button) != 3 || len(message.Button[0]) != 2 {
		t.Fatalf("unexpected keyboard: %+v", message.Button)
	}
}

func TestEncodeEditPhotoResponseWithMedia(t *testing.T) {
	message, err := (MenuServiceBotCodec{}).EncodeEditPhotoResponse(t.Context(), &botv1.EditPhotoResponse{
		PhotoUrl: "https://example.com/b.jpg",
		Caption:  "swapped",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Media == nil {
		t.Fatal("swap edit must carry new media")
	}
}

func TestEncodeHelpResponse(t *testing.T) {
	message, err := (MenuServiceBotCodec{}).EncodeHelpResponse(t.Context(), &botv1.HelpResponse{
		Text: "help text",
		Url:  "https://example.com",
	})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	btn := message.Button[0][0]
	if btn.URL != "https://example.com" {
		t.Fatalf("url button = %+v, want url https://example.com", btn)
	}
	if btn.CallbackData != "" {
		t.Fatalf("url button must not carry callback data: %q", btn.CallbackData)
	}
	// URL row plus the shared home row.
	if len(message.Button) != 2 || !strings.HasPrefix(message.Button[1][0].CallbackData, "nav_home:") {
		t.Fatalf("help must carry the home row: %+v", message.Button)
	}
}

func TestDecodeUpdateCountFromCallback(t *testing.T) {
	codec := MenuServiceBotCodec{}
	// Build the same callback data the "-1" button produces, then decode it back
	// through the callback path.
	callbackData, err := telegram.MarshalData("count", botv1.UpdateCountRequest{Value: 10, Offset: -1})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	update := &telegram.Update{CallbackQuery: &models.CallbackQuery{Data: callbackData}}
	req, err := codec.DecodeUpdateCountRequest(t.Context(), update)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.Value != 10 || req.Offset != -1 {
		t.Fatalf("decoded = %+v, want value=10 offset=-1", req)
	}
}

func TestDecodeUpdateCountFallsBackToDefault(t *testing.T) {
	// A callback with malformed or empty data decodes to the zero request
	// instead of failing, so stray button presses stay harmless.
	codec := MenuServiceBotCodec{}
	req, err := codec.DecodeUpdateCountRequest(t.Context(), &telegram.Update{
		CallbackQuery: &models.CallbackQuery{Data: "count:"},
	})
	if err != nil {
		t.Fatalf("decode with empty payload must not fail: %v", err)
	}
	if req.Value != 0 {
		t.Fatalf("decoded = %+v, want zero request", req)
	}
}

func TestDecodeEditMenuFromCallback(t *testing.T) {
	codec := MenuServiceBotCodec{}
	callbackData, err := telegram.MarshalData("menu", botv1.EditMenuRequest{Action: "pick_0", Size: 2})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := codec.DecodeEditMenuRequest(t.Context(), &telegram.Update{
		CallbackQuery: &models.CallbackQuery{Data: callbackData},
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.Action != "pick_0" || req.Size != 2 {
		t.Fatalf("decoded = %+v, want action=pick_0 size=2", req)
	}
}
