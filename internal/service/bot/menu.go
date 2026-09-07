package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
)

var _ botv1.MenuServiceBotServer = (*Service)(nil)

// photoURLs is a small pool of demo photos cycled by the "swap" button. Use
// images that Telegram can reach over HTTPS; any public image URL works.
var photoURLs = []string{
	"https://picsum.photos/id/1015/800/500",
	"https://picsum.photos/id/1039/800/500",
	"https://picsum.photos/id/1043/800/500",
}

var captions = []string{
	"Photo one — a mountain river.",
	"Photo two — still water.",
	"Photo three — city lights.",
}

// MenuItems are the pickable entries of the /menu card; the codec labels the
// item buttons with them and the keyboard shows menuSize items at a time.
var MenuItems = []string{"🍎 Apple", "🍌 Banana", "🍇 Grape"}

const (
	menuSize     = int64(2) // default item-button count of the /menu card
	menuPhotoURL = "https://picsum.photos/id/1084/800/500"

	catalogPageSize = int64(3) // entries per /catalog page
	catalogMaxCount = int64(5) // quantity stepper upper bound
)

// CatalogEntries is the demo catalog. The list page shows catalogPageSize
// entries at a time with prev/next pagination; a real service would page
// over database rows instead of slicing this slice.
var CatalogEntries = []*botv1.CatalogEntry{
	{Id: 1, Name: "📦 Starter pack", Description: "Everything you need to get started."},
	{Id: 2, Name: "🪙 Coin bundle", Description: "A bundle of in-app coins."},
	{Id: 3, Name: "🎴 Card pack", Description: "A random pack of trading cards."},
	{Id: 4, Name: "⚡ Power-up", Description: "A one-time power-up for your account."},
	{Id: 5, Name: "🛡 Shield", Description: "Protect your account for 30 days."},
	{Id: 6, Name: "💎 Premium", Description: "Premium tier with extra features."},
	{Id: 7, Name: "🎁 Mystery box", Description: "A surprise item from the shop."},
}

func (*Service) Home(_ context.Context, _ *botv1.HomeRequest) (*botv1.HomeResponse, error) {
	// The home screen is static; the codec renders the banner and the demo
	// navigation buttons.
	return &botv1.HomeResponse{}, nil
}

func (*Service) Counter(_ context.Context, _ *botv1.CounterRequest) (*botv1.CounterResponse, error) {
	// A real service would resolve the user from the auth context injected by
	// telegram.NewAuthMiddleware; the counter starts at zero for the demo.
	return &botv1.CounterResponse{Value: 0}, nil
}

func (*Service) UpdateCount(_ context.Context, request *botv1.UpdateCountRequest) (*botv1.UpdateCountResponse, error) {
	return &botv1.UpdateCountResponse{Value: request.Value + request.Offset}, nil
}

func (*Service) ShowPhoto(_ context.Context, _ *botv1.ShowPhotoRequest) (*botv1.ShowPhotoResponse, error) {
	return &botv1.ShowPhotoResponse{
		PhotoUrl: photoURLs[0],
		Caption:  captions[0],
	}, nil
}

func (*Service) EditPhoto(_ context.Context, request *botv1.EditPhotoRequest) (*botv1.EditPhotoResponse, error) {
	// The current message carries the previous value inside the callback data;
	// for a short demo we cycle the pool on every press and stamp the time onto
	// the caption so the in-place edit is visible.
	now := time.Now()
	switch request.Action {
	case "swap":
		return &botv1.EditPhotoResponse{
			PhotoUrl: photoURLs[(now.Second()/10+1)%len(photoURLs)],
			Caption:  fmt.Sprintf("Swapped at %s", now.Format("15:04:05")),
		}, nil
	case "caption":
		return &botv1.EditPhotoResponse{
			Caption: fmt.Sprintf("Caption updated at %s — the photo is unchanged.", now.Format("15:04:05")),
		}, nil
	default:
		return nil, fmt.Errorf("unknown photo action %q", request.Action)
	}
}

func (*Service) ShowMenu(_ context.Context, _ *botv1.ShowMenuRequest) (*botv1.ShowMenuResponse, error) {
	return &botv1.ShowMenuResponse{
		Size:    menuSize,
		Caption: "Menu card — text only, no image yet. Press the buttons below.",
	}, nil
}

func (*Service) EditMenu(_ context.Context, request *botv1.EditMenuRequest) (*botv1.EditMenuResponse, error) {
	// The card is stateless: every callback payload carries the current
	// keyboard size so the response can keep the layout. An empty photo URL
	// renders the card as text-only; "image" attaches the photo, which the
	// sender applies with EditMessageMedia (works on plain text messages too).
	size := min(max(request.Size, 1), int64(len(MenuItems)))
	switch {
	case request.Action == "image":
		return &botv1.EditMenuResponse{
			Size:     size,
			PhotoUrl: menuPhotoURL,
			Caption:  "Menu card — now with an image.",
		}, nil
	case strings.HasPrefix(request.Action, "size_"):
		raw, ok := strings.CutPrefix(request.Action, "size_")
		if !ok {
			return nil, fmt.Errorf("malformed size action %q", request.Action)
		}
		count, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || count < 1 || count > int64(len(MenuItems)) {
			return nil, fmt.Errorf("invalid size action %q", request.Action)
		}
		return &botv1.EditMenuResponse{
			Size:    count,
			Caption: fmt.Sprintf("Menu card — %d item button(s).", count),
		}, nil
	case strings.HasPrefix(request.Action, "pick_"):
		raw, ok := strings.CutPrefix(request.Action, "pick_")
		if !ok {
			return nil, fmt.Errorf("malformed pick action %q", request.Action)
		}
		index, err := strconv.Atoi(raw)
		if err != nil || index < 0 || index >= len(MenuItems) {
			return nil, fmt.Errorf("invalid pick action %q", request.Action)
		}
		return &botv1.EditMenuResponse{
			Size:    size,
			Caption: fmt.Sprintf("You picked %s (keyboard size %d).", MenuItems[index], size),
		}, nil
	default:
		return nil, fmt.Errorf("unknown menu action %q", request.Action)
	}
}

func (*Service) Catalog(_ context.Context, request *botv1.CatalogRequest) (*botv1.CatalogResponse, error) {
	// Page numbers from callback payloads are clamped instead of rejected:
	// prev on the first page and next on the last page simply re-render the
	// same page.
	totalPages := (int64(len(CatalogEntries)) + catalogPageSize - 1) / catalogPageSize
	page := min(max(request.Page, 0), totalPages-1)
	start := page * catalogPageSize
	end := min(start+catalogPageSize, int64(len(CatalogEntries)))
	return &botv1.CatalogResponse{
		Page:       page,
		TotalPages: totalPages,
		Items:      CatalogEntries[start:end],
	}, nil
}

func (*Service) CatalogItem(_ context.Context, request *botv1.CatalogItemRequest) (*botv1.CatalogItemResponse, error) {
	// Unknown ids surface as handler errors, which the bot renders as a toast
	// for callbacks — a stale button from an old list fails visibly.
	if request.Id < 1 || request.Id > int64(len(CatalogEntries)) {
		return nil, fmt.Errorf("unknown catalog item %d", request.Id)
	}
	entry := CatalogEntries[request.Id-1]
	return &botv1.CatalogItemResponse{
		Id:          entry.Id,
		Page:        max(request.Page, 0),
		Count:       min(max(request.Count, 1), catalogMaxCount),
		Name:        entry.Name,
		Description: entry.Description,
	}, nil
}

func (*Service) Help(_ context.Context, _ *botv1.HelpRequest) (*botv1.HelpResponse, error) {
	return &botv1.HelpResponse{
		Text: "Commands:\n/start — home screen with one button per demo\n/counter — counter menu with a changing keyboard\n/menu — card demo: image on/off, 1-3 item buttons\n/photo — media message with in-place edits\n/catalog — paginated list with detail and quantity stepper\n/help — this message",
		Url:  "https://github.com/go-sphere",
	}, nil
}
