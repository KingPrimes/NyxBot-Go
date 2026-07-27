package onebot

import (
	"errors"
	"fmt"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// ReplyText 向当前事件所在的群聊或私聊发送文本。
func ReplyText(ctx *zero.Ctx, text string) error {
	if ctx == nil || ctx.Event == nil {
		return errors.New("message context is unavailable")
	}
	if ctx.Send(text).ID() == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}

// ReplyImage 向当前事件所在的群聊或私聊发送图片字节。
func ReplyImage(ctx *zero.Ctx, image []byte) error {
	if ctx == nil || ctx.Event == nil {
		return errors.New("message context is unavailable")
	}
	if len(image) == 0 {
		return errors.New("image is empty")
	}
	content := message.Message{message.ImageBytes(image)}
	if ctx.Send(content).ID() == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}

// SendGroupText 通过指定在线 Bot 主动发送群文本消息。
func SendGroupText(botUID, groupUID int64, text string) error {
	bot := zero.GetBot(botUID)
	if bot == nil {
		return fmt.Errorf("Bot %d is offline", botUID)
	}
	if bot.SendGroupMessage(groupUID, text) == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}

// SendPrivateText 通过指定在线 Bot 主动发送私聊文本消息。
func SendPrivateText(botUID, userUID int64, text string) error {
	bot := zero.GetBot(botUID)
	if bot == nil {
		return fmt.Errorf("Bot %d is offline", botUID)
	}
	if bot.SendPrivateMessage(userUID, text) == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}

// SendGroupImage 通过指定在线 Bot 主动发送群图片消息。
func SendGroupImage(botUID, groupUID int64, image []byte) error {
	if len(image) == 0 {
		return errors.New("image is empty")
	}
	bot := zero.GetBot(botUID)
	if bot == nil {
		return fmt.Errorf("Bot %d is offline", botUID)
	}
	content := message.Message{message.ImageBytes(image)}
	if bot.SendGroupMessage(groupUID, content) == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}

// SendPrivateImage 通过指定在线 Bot 主动发送私聊图片消息。
func SendPrivateImage(botUID, userUID int64, image []byte) error {
	if len(image) == 0 {
		return errors.New("image is empty")
	}
	bot := zero.GetBot(botUID)
	if bot == nil {
		return fmt.Errorf("Bot %d is offline", botUID)
	}
	content := message.Message{message.ImageBytes(image)}
	if bot.SendPrivateMessage(userUID, content) == 0 {
		return errors.New("OneBot did not return a message id")
	}
	return nil
}
