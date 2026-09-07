package bot

import (
	"context"
	"fmt"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
	botservice "github.com/go-sphere/sphere-telegram-layout/internal/service/bot"
	"github.com/go-sphere/telegram-bot/telegram"
	"github.com/go-telegram/bot/models"
)

var _ botv1.MenuServiceBotCodec = (*MenuServiceBotCodec)(nil)

type MenuServiceBotCodec struct{}

// homeBannerURL is the banner shown on the home screen. The home screen
// always carries media so that navigating back from a photo screen replaces
// the previous image instead of leaving it behind.
const homeBannerURL = "https://picsum.photos/id/180/800/500"

// newHomeButton builds the "Home" button every demo screen carries; the
// "nav_home" callback edits the current message back into the home screen.
func newHomeButton() (telegram.Button, error) {
	return NewButtonX("🏠 Home", botv1.ExtraBotDataMenuServiceHome, botv1.HomeRequest{})
}

// appendHomeButton adds the shared home row to a rendered screen.
func appendHomeButton(msg *telegram.Message) (*telegram.Message, error) {
	home, err := newHomeButton()
	if err != nil {
		return nil, err
	}
	msg.Button = append(msg.Button, []telegram.Button{home})
	return msg, nil
}

// --- /start home ---------------------------------------------------------

func (MenuServiceBotCodec) DecodeHomeRequest(_ context.Context, _ *telegram.Update) (*botv1.HomeRequest, error) {
	return &botv1.HomeRequest{}, nil
}

func (MenuServiceBotCodec) EncodeHomeResponse(_ context.Context, _ *botv1.HomeResponse) (*telegram.Message, error) {
	// One button per demo. Each "nav_*"/"catalog" callback is bound to the
	// same RPC as the matching command, so pressing a button edits the home
	// message in place into the demo screen.
	counter, err := NewButtonX("🔢 Counter", botv1.ExtraBotDataMenuServiceCounter, botv1.CounterRequest{})
	if err != nil {
		return nil, err
	}
	catalog, err := NewButtonX("📦 Catalog", botv1.ExtraBotDataMenuServiceCatalog, botv1.CatalogRequest{})
	if err != nil {
		return nil, err
	}
	card, err := NewButtonX("🃏 Menu card", botv1.ExtraBotDataMenuServiceShowMenu, botv1.ShowMenuRequest{})
	if err != nil {
		return nil, err
	}
	photo, err := NewButtonX("🖼 Photo", botv1.ExtraBotDataMenuServiceShowPhoto, botv1.ShowPhotoRequest{})
	if err != nil {
		return nil, err
	}
	help, err := NewButtonX("❓ Help", botv1.ExtraBotDataMenuServiceHelp, botv1.HelpRequest{})
	if err != nil {
		return nil, err
	}
	return &telegram.Message{
		Media: telegram.NewStringInputFile(homeBannerURL),
		Text:  "🏠 Sphere Bot\nPick a demo:",
		Button: [][]telegram.Button{
			{counter, catalog},
			{card, photo},
			{help},
		},
	}, nil
}

// --- /counter ------------------------------------------------------------

func (MenuServiceBotCodec) DecodeCounterRequest(_ context.Context, _ *telegram.Update) (*botv1.CounterRequest, error) {
	return &botv1.CounterRequest{}, nil
}

func (MenuServiceBotCodec) EncodeCounterResponse(_ context.Context, response *botv1.CounterResponse) (*telegram.Message, error) {
	msg, err := encodeCounterMessage(response.Value)
	if err != nil {
		return nil, err
	}
	return appendHomeButton(msg)
}

// encodeCounterMessage renders the counter menu. The keyboard changes with the
// value: a single "+1" button at zero, and "-1/+1" plus "Reset" once the
// counter has moved — inline keyboards with 1 and 3 buttons from the same
// handler.
func encodeCounterMessage(value int64) (*telegram.Message, error) {
	// The counter buttons carry a typed payload: the current value plus the
	// offset to apply when pressed.
	plus, err := NewButtonX("+1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{Value: value, Offset: 1})
	if err != nil {
		return nil, err
	}
	if value == 0 {
		return &telegram.Message{
			Text:   fmt.Sprintf("Counter: %d", value),
			Button: [][]telegram.Button{{plus}},
		}, nil
	}
	minus, err := NewButtonX("-1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{Value: value, Offset: -1})
	if err != nil {
		return nil, err
	}
	reset, err := NewButtonX("Reset", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{Value: 0, Offset: 0})
	if err != nil {
		return nil, err
	}
	return &telegram.Message{
		Text: fmt.Sprintf("Counter: %d", value),
		Button: [][]telegram.Button{
			{minus, plus},
			{reset},
		},
	}, nil
}

// --- count: callback --------------------------------------------------------

func (MenuServiceBotCodec) DecodeUpdateCountRequest(_ context.Context, request *telegram.Update) (*botv1.UpdateCountRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.UpdateCountRequest{})
}

func (MenuServiceBotCodec) EncodeUpdateCountResponse(_ context.Context, response *botv1.UpdateCountResponse) (*telegram.Message, error) {
	// Same keyboard as /counter: the callback sender (SendMessage) edits the
	// original message in place, so the counter updates without spamming the
	// chat, and the keyboard shrinks back to a single button when the counter
	// returns to zero.
	msg, err := encodeCounterMessage(response.Value)
	if err != nil {
		return nil, err
	}
	return appendHomeButton(msg)
}

// --- /photo --------------------------------------------------------------

func (MenuServiceBotCodec) DecodeShowPhotoRequest(_ context.Context, _ *telegram.Update) (*botv1.ShowPhotoRequest, error) {
	return &botv1.ShowPhotoRequest{}, nil
}

func (MenuServiceBotCodec) EncodeShowPhotoResponse(_ context.Context, response *botv1.ShowPhotoResponse) (*telegram.Message, error) {
	swap, err := NewButtonX("Swap photo", botv1.ExtraBotDataMenuServiceEditPhoto, botv1.EditPhotoRequest{Action: "swap"})
	if err != nil {
		return nil, err
	}
	next, err := NewButtonX("Next caption", botv1.ExtraBotDataMenuServiceEditPhoto, botv1.EditPhotoRequest{Action: "caption"})
	if err != nil {
		return nil, err
	}
	// Media + text: for a fresh /photo this sends a photo message; pressing a
	// button re-renders the same Message through the callback path, where the
	// sender edits the existing media message (swap -> EditMessageMedia,
	// caption -> EditMessageCaption).
	msg := &telegram.Message{
		Media:     telegram.NewStringInputFile(response.PhotoUrl),
		Text:      response.Caption,
		ParseMode: models.ParseModeHTML,
		Button: [][]telegram.Button{
			{swap, next},
		},
	}
	return appendHomeButton(msg)
}

func (MenuServiceBotCodec) DecodeEditPhotoRequest(_ context.Context, request *telegram.Update) (*botv1.EditPhotoRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.EditPhotoRequest{})
}

func (MenuServiceBotCodec) EncodeEditPhotoResponse(_ context.Context, response *botv1.EditPhotoResponse) (*telegram.Message, error) {
	swap, err := NewButtonX("Swap photo", botv1.ExtraBotDataMenuServiceEditPhoto, botv1.EditPhotoRequest{Action: "swap"})
	if err != nil {
		return nil, err
	}
	next, err := NewButtonX("Next caption", botv1.ExtraBotDataMenuServiceEditPhoto, botv1.EditPhotoRequest{Action: "caption"})
	if err != nil {
		return nil, err
	}
	// The response carries either a new photo URL (swap) or just a new caption,
	// depending on which button was pressed. With the caption action the media
	// stays nil so the sender edits only the caption.
	msg := &telegram.Message{
		Media:     photoIfNotEmpty(response.PhotoUrl),
		Text:      response.Caption,
		ParseMode: models.ParseModeHTML,
		Button: [][]telegram.Button{
			{swap, next},
		},
	}
	return appendHomeButton(msg)
}

// photoIfNotEmpty maps an empty photo URL to nil so a response that only
// changes the caption does not carry media.
func photoIfNotEmpty(url string) models.InputFile {
	if url == "" {
		return nil
	}
	return telegram.NewStringInputFile(url)
}

// --- /menu ---------------------------------------------------------------

func (MenuServiceBotCodec) DecodeShowMenuRequest(_ context.Context, _ *telegram.Update) (*botv1.ShowMenuRequest, error) {
	return &botv1.ShowMenuRequest{}, nil
}

func (MenuServiceBotCodec) EncodeShowMenuResponse(_ context.Context, response *botv1.ShowMenuResponse) (*telegram.Message, error) {
	return encodeMenuMessage(response.Size, response.PhotoUrl, response.Caption)
}

func (MenuServiceBotCodec) DecodeEditMenuRequest(_ context.Context, request *telegram.Update) (*botv1.EditMenuRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.EditMenuRequest{})
}

func (MenuServiceBotCodec) EncodeEditMenuResponse(_ context.Context, response *botv1.EditMenuResponse) (*telegram.Message, error) {
	return encodeMenuMessage(response.Size, response.PhotoUrl, response.Caption)
}

// encodeMenuMessage renders the /menu card: one row of item buttons (1-3,
// depending on size) above a fixed control row and the shared home row. With
// an empty photo URL the card is text-only; with a URL the sender sends a
// photo for commands or edits the message with EditMessageMedia for
// callbacks, which upgrades a plain text card in place.
func encodeMenuMessage(size int64, photoURL, caption string) (*telegram.Message, error) {
	itemRow := make([]telegram.Button, 0, size)
	for i := range size {
		button, err := NewButtonX(botservice.MenuItems[i], botv1.ExtraBotDataMenuServiceEditMenu, botv1.EditMenuRequest{
			Action: fmt.Sprintf("pick_%d", i),
			Size:   size,
		})
		if err != nil {
			return nil, err
		}
		itemRow = append(itemRow, button)
	}
	controlRow := make([]telegram.Button, 0, 4)
	for _, control := range []struct {
		label  string
		action string
	}{
		{"🖼 Image", "image"},
		{"1 btn", "size_1"},
		{"2 btn", "size_2"},
		{"3 btn", "size_3"},
	} {
		button, err := NewButtonX(control.label, botv1.ExtraBotDataMenuServiceEditMenu, botv1.EditMenuRequest{
			Action: control.action,
			Size:   size,
		})
		if err != nil {
			return nil, err
		}
		controlRow = append(controlRow, button)
	}
	msg := &telegram.Message{
		Media: photoIfNotEmpty(photoURL),
		Text:  caption,
		Button: [][]telegram.Button{
			itemRow,
			controlRow,
		},
	}
	return appendHomeButton(msg)
}

// --- /catalog ------------------------------------------------------------

func (MenuServiceBotCodec) DecodeCatalogRequest(_ context.Context, request *telegram.Update) (*botv1.CatalogRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.CatalogRequest{})
}

func (MenuServiceBotCodec) EncodeCatalogResponse(_ context.Context, response *botv1.CatalogResponse) (*telegram.Message, error) {
	// One row per entry; each item button carries the item id plus the
	// originating page so the detail page can navigate back to this page.
	rows := make([][]telegram.Button, 0, len(response.Items)+2)
	for _, item := range response.Items {
		button, err := NewButtonX(item.Name, botv1.ExtraBotDataMenuServiceCatalogItem, botv1.CatalogItemRequest{
			Id:    item.Id,
			Page:  response.Page,
			Count: 1,
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, []telegram.Button{button})
	}
	// Pagination row: prev/next are hidden on the first/last page; the page
	// indicator re-renders the current page.
	pageRow := make([]telegram.Button, 0, 3)
	if response.Page > 0 {
		prev, err := NewButtonX("⏪ Prev", botv1.ExtraBotDataMenuServiceCatalog, botv1.CatalogRequest{Page: response.Page - 1})
		if err != nil {
			return nil, err
		}
		pageRow = append(pageRow, prev)
	}
	indicator, err := NewButtonX(fmt.Sprintf("%d/%d", response.Page+1, response.TotalPages), botv1.ExtraBotDataMenuServiceCatalog, botv1.CatalogRequest{Page: response.Page})
	if err != nil {
		return nil, err
	}
	pageRow = append(pageRow, indicator)
	if response.Page+1 < response.TotalPages {
		next, err := NewButtonX("Next ⏩", botv1.ExtraBotDataMenuServiceCatalog, botv1.CatalogRequest{Page: response.Page + 1})
		if err != nil {
			return nil, err
		}
		pageRow = append(pageRow, next)
	}
	rows = append(rows, pageRow)
	return appendHomeButton(&telegram.Message{
		Text:   "Catalog 📦",
		Button: rows,
	})
}

func (MenuServiceBotCodec) DecodeCatalogItemRequest(_ context.Context, request *telegram.Update) (*botv1.CatalogItemRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.CatalogItemRequest{})
}

func (MenuServiceBotCodec) EncodeCatalogItemResponse(_ context.Context, response *botv1.CatalogItemResponse) (*telegram.Message, error) {
	// Quantity stepper: every button carries the item id, the originating
	// page, and the new count; the service clamps the count to its bounds.
	minus, err := NewButtonX("−", botv1.ExtraBotDataMenuServiceCatalogItem, botv1.CatalogItemRequest{
		Id:    response.Id,
		Page:  response.Page,
		Count: response.Count - 1,
	})
	if err != nil {
		return nil, err
	}
	count, err := NewButtonX(fmt.Sprintf("%d", response.Count), botv1.ExtraBotDataMenuServiceCatalogItem, botv1.CatalogItemRequest{
		Id:    response.Id,
		Page:  response.Page,
		Count: response.Count,
	})
	if err != nil {
		return nil, err
	}
	plus, err := NewButtonX("+", botv1.ExtraBotDataMenuServiceCatalogItem, botv1.CatalogItemRequest{
		Id:    response.Id,
		Page:  response.Page,
		Count: response.Count + 1,
	})
	if err != nil {
		return nil, err
	}
	// Back to the parent page: the "catalog" callback is the same RPC as the
	// /catalog command, so this re-renders the originating list page.
	back, err := NewButtonX("↩ Back", botv1.ExtraBotDataMenuServiceCatalog, botv1.CatalogRequest{Page: response.Page})
	if err != nil {
		return nil, err
	}
	return appendHomeButton(&telegram.Message{
		Text: fmt.Sprintf("%s — %s\nQuantity: %d", response.Name, response.Description, response.Count),
		Button: [][]telegram.Button{
			{minus, count, plus},
			{back},
		},
	})
}

// --- /help ---------------------------------------------------------------

func (MenuServiceBotCodec) DecodeHelpRequest(_ context.Context, _ *telegram.Update) (*botv1.HelpRequest, error) {
	return &botv1.HelpRequest{}, nil
}

func (MenuServiceBotCodec) EncodeHelpResponse(_ context.Context, response *botv1.HelpResponse) (*telegram.Message, error) {
	// A plain text reply with an inline keyboard containing a URL button (no
	// callback data, so Telegram opens the link when pressed) plus the shared
	// home row.
	msg := &telegram.Message{
		Text: response.Text,
		Button: [][]telegram.Button{
			{telegram.NewURLButton("Open website", response.Url)},
		},
	}
	return appendHomeButton(msg)
}
