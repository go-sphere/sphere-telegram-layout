package bot

import "github.com/go-sphere/telegram-bot/telegram"

func NewButtonX[T any](text string, extra *telegram.MethodExtraData, data T) telegram.Button {
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
