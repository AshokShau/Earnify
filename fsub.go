package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

var (
	chatInviteLinks = make(map[int64]string)
	chatCacheMutex  sync.RWMutex
	memberStatuses  = map[string]bool{
		"member":        true,
		"administrator": true,
		"creator":       true,
	}
)

func retryMarkup(b *gotgbot.Bot, args, link string) *gotgbot.InlineKeyboardMarkup {
	buttons := [][]gotgbot.InlineKeyboardButton{
		{
			{Text: "Jᴏɪɴ", Url: link},
		},
	}

	if args != "" {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: "Tʀʏ ᴀɢᴀɪɴ", Url: fmt.Sprintf("https://t.me/%s?start=%s", b.Username, args)},
		})
	}

	return &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func fetchInviteLink(b *gotgbot.Bot, chatID int64) (string, error) {
	chatCacheMutex.RLock()
	if link, found := chatInviteLinks[chatID]; found {
		chatCacheMutex.RUnlock()
		return link, nil
	}
	chatCacheMutex.RUnlock()

	chatCacheMutex.Lock()
	defer chatCacheMutex.Unlock()

	if link, found := chatInviteLinks[chatID]; found {
		return link, nil
	}

	chat, err := b.GetChat(chatID, nil)
	if err != nil {
		log.Printf("Error getting chat: %s", err)
		return "", err
	}

	chatInviteLinks[chatID] = chat.InviteLink
	return chat.InviteLink, nil
}

func fSub(b *gotgbot.Bot, userId int64, arg string) (bool, error) {
	chats := FSubIds
	if len(chats) == 0 {
		log.Print("FSub IDs not set")
		return true, nil
	}

	for _, chatID := range chats {
		userMember, err := b.GetChatMember(chatID, userId, nil)
		if err != nil {
			return false, fmt.Errorf("error getting chat member: %s", err)
		}

		mem := userMember.MergeChatMember()
		if !memberStatuses[mem.Status] {
			inviteLink, err := fetchInviteLink(b, chatID)
			if err != nil || inviteLink == "" {
				return false, fmt.Errorf("invite link not available")
			}

			btn := retryMarkup(b, arg, inviteLink)
			_, err = b.SendMessage(userId, "❌ You must be a member of the channel to use this bot.\nPlease join the channel and try again.", &gotgbot.SendMessageOpts{
				ReplyMarkup: btn,
			})

			if err != nil {
				log.Printf("Error sending message: %s", err)
			}

			return false, nil
		}
	}

	return true, nil
}
