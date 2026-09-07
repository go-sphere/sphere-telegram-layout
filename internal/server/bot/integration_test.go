package bot

import (
	"context"
	"strings"
	"sync"
	"testing"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
	botservice "github.com/go-sphere/sphere-telegram-layout/internal/service/bot"
	"github.com/go-sphere/telegram-bot/telegram"
	"github.com/go-telegram/bot/models"
)

// capture is a fake render function that records the last rendered message,
// mirroring what telegram.SendMessage does with real updates.
type capture struct {
	mu      sync.Mutex
	message *telegram.Message
}

func (c *capture) render(_ context.Context, _ *telegram.Update, msg *telegram.Message) error {
	c.mu.Lock()
	c.message = msg
	c.mu.Unlock()
	return nil
}

func (c *capture) last() *telegram.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.message
}

// runOperation drives one generated bot handler with the given update and
// returns the message rendered into cap (the render function the handlers were
// registered with).
func runOperation(t *testing.T, handlers telegram.RouteMap, cap *capture, operation string, update *telegram.Update) *telegram.Message {
	t.Helper()
	fn := handlers[operation]
	if fn == nil {
		t.Fatalf("operation %q not registered", operation)
	}
	if err := fn(context.Background(), update); err != nil {
		t.Fatalf("handler %q: %v", operation, err)
	}
	return cap.last()
}

func callbackUpdate(data string) *telegram.Update {
	return &telegram.Update{CallbackQuery: &models.CallbackQuery{ID: "cq", Data: data}}
}

func messageUpdate(text string) *telegram.Update {
	return &telegram.Update{Message: &models.Message{Text: text}}
}

func TestBotRouteIntegration(t *testing.T) {
	srv := botservice.NewService()
	codec := &MenuServiceBotCodec{}
	cap := &capture{}
	handlers := botv1.RegisterMenuServiceBotServer(srv, codec, cap.render)

	// Any operation handler must be registered (the generated map is complete).
	for _, op := range botv1.GetAllBotMenuServiceOperations() {
		if handlers[op] == nil {
			t.Fatalf("operation %q missing from handler map", op)
		}
	}

	t.Run("start renders home with navigation grid", func(t *testing.T) {
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceHome, messageUpdate("/start"))
		if msg == nil || msg.Media == nil {
			t.Fatalf("home must render the banner media, got %+v", msg)
		}
		// Grid with one button per demo (2 + 2 + 1 rows).
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 || len(msg.Button[1]) != 2 || len(msg.Button[2]) != 1 {
			t.Fatalf("unexpected home keyboard: %+v", msg.Button)
		}
		for _, want := range []string{"nav_counter:", "catalog:", "nav_menu:", "nav_photo:", "nav_help:"} {
			found := false
			for _, row := range msg.Button {
				for _, button := range row {
					if strings.HasPrefix(button.CallbackData, want) {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("home keyboard missing button with route %q: %+v", want, msg.Button)
			}
		}
	})

	t.Run("nav_counter callback renders counter screen", func(t *testing.T) {
		data, err := telegram.MarshalData("nav_counter", botv1.CounterRequest{})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceCounter, callbackUpdate(data))
		if msg == nil || msg.Text != "Counter: 0" {
			t.Fatalf("counter message = %+v", msg)
		}
		// A zero counter is text-only and shows just the "+1" button plus the
		// home row; the keyboard grows once the counter moves.
		if msg.Media != nil {
			t.Fatalf("counter must be text-only, got %T", msg.Media)
		}
		if len(msg.Button) != 2 || len(msg.Button[0]) != 1 {
			t.Fatalf("unexpected counter keyboard: %+v", msg.Button)
		}
	})

	t.Run("nav_home callback returns to home", func(t *testing.T) {
		data, err := telegram.MarshalData("nav_home", botv1.HomeRequest{})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceHome, callbackUpdate(data))
		if msg == nil || msg.Media == nil {
			t.Fatalf("home must render the banner media, got %+v", msg)
		}
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 {
			t.Fatalf("unexpected home keyboard: %+v", msg.Button)
		}
	})

	t.Run("count callback increments and grows the keyboard", func(t *testing.T) {
		data, err := telegram.MarshalData("count", botv1.UpdateCountRequest{Value: 0, Offset: 1})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceUpdateCount, callbackUpdate(data))
		if msg == nil || msg.Text != "Counter: 1" {
			t.Fatalf("count callback message = %+v", msg)
		}
		// Counter rows (-1/+1, Reset) plus the home row.
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 || len(msg.Button[1]) != 1 {
			t.Fatalf("moved counter must show 3 buttons plus home: %+v", msg.Button)
		}
	})

	t.Run("menu command renders text-only card", func(t *testing.T) {
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceShowMenu, messageUpdate("/menu"))
		if msg == nil {
			t.Fatal("nil message")
		}
		if msg.Media != nil {
			t.Fatalf("menu card must start text-only, got %T", msg.Media)
		}
		// Default size 2: item row (2) + control row (image, 1, 2, 3) + home.
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 || len(msg.Button[1]) != 4 {
			t.Fatalf("unexpected menu keyboard: %+v", msg.Button)
		}
	})

	t.Run("menu image action renders media", func(t *testing.T) {
		data, err := telegram.MarshalData("menu", botv1.EditMenuRequest{Action: "image", Size: 2})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceEditMenu, callbackUpdate(data))
		if msg == nil || msg.Media == nil {
			t.Fatalf("image action must carry media, got %+v", msg)
		}
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 {
			t.Fatalf("image action must keep the keyboard size: %+v", msg.Button)
		}
	})

	t.Run("menu size action changes the keyboard", func(t *testing.T) {
		data, err := telegram.MarshalData("menu", botv1.EditMenuRequest{Action: "size_3", Size: 2})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceEditMenu, callbackUpdate(data))
		if msg == nil {
			t.Fatal("nil message")
		}
		// Size 3: item row (3) + control row (4) + home; still text-only.
		if len(msg.Button) != 3 || len(msg.Button[0]) != 3 || len(msg.Button[1]) != 4 {
			t.Fatalf("unexpected menu keyboard: %+v", msg.Button)
		}
		if msg.Media != nil {
			t.Fatalf("size action must not carry media, got %T", msg.Media)
		}
	})

	t.Run("menu pick keeps size and renders caption", func(t *testing.T) {
		data, err := telegram.MarshalData("menu", botv1.EditMenuRequest{Action: "pick_0", Size: 2})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceEditMenu, callbackUpdate(data))
		if msg == nil {
			t.Fatal("nil message")
		}
		if msg.Media != nil {
			t.Fatalf("pick must be media-free, got %T", msg.Media)
		}
		if !strings.Contains(msg.Text, "You picked") {
			t.Fatalf("unexpected pick caption: %q", msg.Text)
		}
		if len(msg.Button) != 3 || len(msg.Button[0]) != 2 {
			t.Fatalf("pick must keep the keyboard size: %+v", msg.Button)
		}
	})

	t.Run("catalog command renders first page", func(t *testing.T) {
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceCatalog, messageUpdate("/catalog"))
		if msg == nil {
			t.Fatal("nil message")
		}
		// 3 item rows + pagination row (no prev on page 1) + home row.
		if len(msg.Button) != 5 {
			t.Fatalf("unexpected catalog keyboard: %+v", msg.Button)
		}
		pageRow := msg.Button[3]
		if len(pageRow) != 2 || pageRow[0].Text != "1/3" || pageRow[1].Text != "Next ⏩" {
			t.Fatalf("unexpected pagination row: %+v", pageRow)
		}
		if !strings.Contains(msg.Button[0][0].Text, "Starter pack") {
			t.Fatalf("first row = %q, want the first catalog entry", msg.Button[0][0].Text)
		}
	})

	t.Run("catalog next page callback paginates", func(t *testing.T) {
		data, err := telegram.MarshalData("catalog", botv1.CatalogRequest{Page: 1})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceCatalog, callbackUpdate(data))
		if msg == nil {
			t.Fatal("nil message")
		}
		// Page 2 of 3: prev and next are both visible, entries 4-6 listed.
		pageRow := msg.Button[3]
		if len(pageRow) != 3 || pageRow[0].Text != "⏪ Prev" || pageRow[1].Text != "2/3" || pageRow[2].Text != "Next ⏩" {
			t.Fatalf("unexpected pagination row: %+v", pageRow)
		}
		if !strings.Contains(msg.Button[0][0].Text, "Power-up") {
			t.Fatalf("first row = %q, want the fourth catalog entry", msg.Button[0][0].Text)
		}
	})

	t.Run("catalog item callback renders detail with stepper", func(t *testing.T) {
		data, err := telegram.MarshalData("catalog_item", botv1.CatalogItemRequest{Id: 2, Page: 1, Count: 1})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceCatalogItem, callbackUpdate(data))
		if msg == nil {
			t.Fatal("nil message")
		}
		if !strings.Contains(msg.Text, "Coin bundle") || !strings.Contains(msg.Text, "Quantity: 1") {
			t.Fatalf("unexpected detail text: %q", msg.Text)
		}
		// Stepper row, back row (catalog route), home row.
		if len(msg.Button) != 3 || len(msg.Button[0]) != 3 {
			t.Fatalf("unexpected detail keyboard: %+v", msg.Button)
		}
		if !strings.HasPrefix(msg.Button[1][0].CallbackData, "catalog:") {
			t.Fatalf("back button must carry the catalog route: %q", msg.Button[1][0].CallbackData)
		}
	})

	t.Run("catalog item plus button raises the quantity", func(t *testing.T) {
		data, err := telegram.MarshalData("catalog_item", botv1.CatalogItemRequest{Id: 2, Page: 0, Count: 2})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceCatalogItem, callbackUpdate(data))
		if msg == nil || !strings.Contains(msg.Text, "Quantity: 2") {
			t.Fatalf("unexpected detail text: %+v", msg)
		}
	})

	t.Run("unknown catalog item errors", func(t *testing.T) {
		if _, err := srv.CatalogItem(t.Context(), &botv1.CatalogItemRequest{Id: 99}); err == nil {
			t.Fatal("expected an error for an unknown catalog item")
		}
	})

	t.Run("photo command renders media message", func(t *testing.T) {
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceShowPhoto, messageUpdate("/photo"))
		if msg == nil || msg.Media == nil {
			t.Fatalf("photo command must render media, got %+v", msg)
		}
		if _, ok := msg.Media.(*models.InputFileString); !ok {
			t.Fatalf("photo media = %T, want URL input", msg.Media)
		}
		// Swap/caption row plus the home row.
		if len(msg.Button) != 2 || len(msg.Button[0]) != 2 {
			t.Fatalf("unexpected photo keyboard: %+v", msg.Button)
		}
	})

	t.Run("caption action edits caption without media", func(t *testing.T) {
		data, err := telegram.MarshalData("photo", botv1.EditPhotoRequest{Action: "caption"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceEditPhoto, callbackUpdate(data))
		if msg == nil {
			t.Fatal("nil message")
		}
		if msg.Media != nil {
			t.Fatalf("caption-only edit must not carry media, got %T", msg.Media)
		}
		if !strings.Contains(msg.Text, "Caption updated") {
			t.Fatalf("unexpected caption: %q", msg.Text)
		}
	})

	t.Run("swap action replaces media", func(t *testing.T) {
		data, err := telegram.MarshalData("photo", botv1.EditPhotoRequest{Action: "swap"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceEditPhoto, callbackUpdate(data))
		if msg == nil || msg.Media == nil {
			t.Fatalf("swap action must carry new media, got %+v", msg)
		}
	})

	t.Run("help renders URL button", func(t *testing.T) {
		msg := runOperation(t, handlers, cap, botv1.OperationBotMenuServiceHelp, messageUpdate("/help"))
		if msg == nil {
			t.Fatal("nil message")
		}
		btn := msg.Button[0][0]
		if btn.URL == "" || btn.CallbackData != "" {
			t.Fatalf("help button = %+v, want URL-only button", btn)
		}
	})
}
