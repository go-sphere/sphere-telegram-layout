package bot

import "github.com/go-sphere/telegram-bot/telegram"

// NewButtonX builds an inline keyboard button carrying generated route metadata.
// The route comes from the generated service contract (ExtraBotData*), and the
// payload is produced by telegram.MarshalData, so the marshaled callback data
// must fit Telegram's 64-byte limit. The error is surfaced rather than
// swallowed, so an oversized payload fails at encode time instead of at the
// Telegram API.
func NewButtonX[T any](text string, extra *telegram.MethodExtraData, data T) (telegram.Button, error) {
	return telegram.NewButton(text, extra.CallbackQuery, data)
}

func UnmarshalUpdateDataWithDefault[T any](update *telegram.Update, defaultValue *T) (*T, error) {
	if update == nil || update.CallbackQuery == nil {
		return defaultValue, nil
	}
	_, data, err := telegram.UnmarshalData[T](update.CallbackQuery.Data)
	if err != nil && defaultValue != nil {
		return defaultValue, nil
	}
	return data, err
}
