package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	_ "github.com/joho/godotenv/autoload"
)

const (
	WITHDWAWL = "Withdrawal"
	SET_ACC   = "SetAccount"
)

var (
	WebhookURL     string
	Port           string
	MongoDBURI     string
	secretToken    string
	OwnerID        int64
	LoggerID       int64
	FSubIds        []int64
	ctx            = context.TODO()
	allowedUpdates = []string{"message", "callback_query"}
)

func main() {
	var err error
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN is not set")
	}

	OwnerID, err = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if err != nil {
		log.Fatal("OWNER_ID is not set")
	}

	LoggerID, err = strconv.ParseInt(os.Getenv("LOGGER_ID"), 10, 64)
	if err != nil {
		log.Fatal("LOGGER_ID is not set")
	}

	fsubids, err := strconv.ParseInt(os.Getenv("FSUB_IDS"), 10, 64)
	if err != nil {
		log.Fatal("FSUB_IDS is not set")
	}

	FSubIds = append(FSubIds, fsubids)
	MongoDBURI = os.Getenv("MONGO_URI")

	secretToken = os.Getenv("SECRET_TOKEN")
	if secretToken == "" {
		secretToken = "OopsNoSECRET_TOKENFoundTimeToCallSherlock"
	}

	WebhookURL = os.Getenv("WEBHOOK_URL")
	Port = os.Getenv("PORT")

	clientOptions := options.Client().ApplyURI(MongoDBURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	fmt.Println("Connected to MongoDB")
	db := client.Database("tgreferearn")
	userColl = db.Collection("users")

	bot, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		BotClient: &gotgbot.BaseBotClient{
			Client: http.Client{},
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: gotgbot.DefaultTimeout,
				APIURL:  gotgbot.DefaultAPIURL,
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	dispatcher.AddHandler(handlers.NewCommand("start", start))
	dispatcher.AddHandler(handlers.NewCommand("help", help))
	dispatcher.AddHandler(handlers.NewCommand("info", info))
	dispatcher.AddHandler(handlers.NewCommand("add", addBalance))
	dispatcher.AddHandler(handlers.NewCommand("remove", removeBalanceCmd))
	dispatcher.AddHandler(handlers.NewCommand("accno", updateAccNo))
	dispatcher.AddHandler(handlers.NewCommand("stats", stats))
	dispatcher.AddHandler(handlers.NewCommand("broadcast", broadcast))

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("info"), infoCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("wallet"), walletCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("confirm_withdrawal"), confirmWithdrawal))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("home"), home))

	dispatcher.AddHandler(handlers.NewConversation(
		[]ext.Handler{handlers.NewCallback(callbackquery.Prefix("withdraw"), withdrawal)},
		map[string][]ext.Handler{
			WITHDWAWL: {handlers.NewMessage(onlyFloat64, withdrawalAsk)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", cancel)},
			StateStorage: conversation.NewInMemoryStorage(conversation.KeyStrategySenderAndChat),
			AllowReEntry: true,
		},
	))

	dispatcher.AddHandler(handlers.NewConversation(
		[]ext.Handler{handlers.NewCallback(callbackquery.Prefix("setAccNo"), setAccNo)},
		map[string][]ext.Handler{
			SET_ACC: {handlers.NewMessage(onlyInt64, setAccAsk)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", cancel)},
			StateStorage: conversation.NewInMemoryStorage(conversation.KeyStrategySenderAndChat),
			AllowReEntry: true,
		},
	))

	updater := ext.NewUpdater(dispatcher, nil)

	if WebhookURL != "" && Port != "" {
		_, err := bot.SetWebhook(WebhookURL+token, &gotgbot.SetWebhookOpts{
			MaxConnections:     40,
			DropPendingUpdates: true,
			SecretToken:        secretToken,
			AllowedUpdates:     allowedUpdates,
		})

		if err != nil {
			panic("failed to set webhook: " + err.Error())
		}

		updater.StartWebhook(bot, token, ext.WebhookOpts{
			ListenAddr:  "0.0.0.0:" + Port,
			SecretToken: secretToken,
		})
	} else {
		err = updater.StartPolling(bot, &ext.PollingOpts{
			DropPendingUpdates: true,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout:        9,
				AllowedUpdates: allowedUpdates,
				RequestOpts: &gotgbot.RequestOpts{
					Timeout: time.Second * 10,
				},
			},
		})

		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("%s has been started...\n", bot.User.Username)
	updater.Idle()
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]

	var userArgs string
	if len(args) > 0 {
		userArgs = args[0]
	} else {
		userArgs = ""
	}

	isMember, err := fSub(b, user.Id, userArgs)
	if err != nil {
		_, _ = msg.Reply(b, "❌ An error occurred. Please try again later.", nil)
		return fmt.Errorf("start: %v", err)
	}

	if !isMember {
		return ext.EndGroups
	}

	referUrl := fmt.Sprintf("https://t.me/%s?start=%d", b.User.Username, user.Id)

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: "👤 Owner",
					Url:  fmt.Sprintf("tg://user?id=%d", OwnerID),
				},
			},
			{
				{
					Text: "🔗 Refer & Earn",
					Url:  fmt.Sprintf("https://t.me/share/url?url=%s", referUrl),
				},
				{
					Text:         "ℹ️ Info",
					CallbackData: fmt.Sprintf("info.%d", user.Id),
				},
			},
			{
				{
					Text:         "💼 Wallet",
					CallbackData: fmt.Sprintf("wallet.%d", user.Id),
				},
				{
					Text:         "💸 Withdraw",
					CallbackData: fmt.Sprintf("withdraw.%d", user.Id),
				},
			},
		},
	}

	existingUser, err := getUser(user.Id)
	if err != nil && err.Error() != "mongo: no documents in result" {
		log.Printf("Failed to fetch user: %v", err)
		_, _ = msg.Reply(b, "❌ An error occurred. Please try again later.\n/start", nil)
		return nil
	}

	if existingUser != nil {
		response := fmt.Sprintf(
			"👋 <b>Welcome back, %s!</b>\n\n"+
				"💰 <b>Balance:</b> %.2f\n"+
				"🤝 <b>Referred Users:</b> %d\n\n"+
				"🚀 Keep earning rewards by referring your friends!",
			user.FirstName, existingUser.Balance, len(existingUser.ReferredUsers))

		_, _ = msg.Reply(b, response, &gotgbot.SendMessageOpts{
			ReplyMarkup: button,
			ParseMode:   "HTML",
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		})

		return nil
	}

	var referrerID int64
	if len(args) > 0 {
		referralCode := strings.TrimSpace(args[0])
		referrerID, err = strconv.ParseInt(referralCode, 10, 64)
		if err != nil || referrerID <= 0 {
			_, _ = msg.Reply(b, "❌ <b>Invalid referral code!</b>\n\nPlease check the code and try again.", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})
			return nil
		}

		referrer, err := getUser(referrerID)
		if err != nil {
			_, _ = msg.Reply(b, "❌ <b>The referral code is not valid.</b>\n\nPlease check with the person who referred you.", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})

			return nil
		}

		log.Printf("Referrer ID: %d", referrer.ID)
		err = referUser(referrerID, user.Id)
		if err != nil {
			log.Printf("Failed to refer user: %v", err)
			_, _ = msg.Reply(b, "⚠️ <b>Failed to register with the referral. Please try again.</b>", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})

			return nil
		}
		_, _ = b.SendMessage(referrerID, fmt.Sprintf(
			"🎉 <b>Referral Successful!</b>\n\n"+
				"👤 You referred <b>%s</b> (%d) successfully!\n"+
				"💵 You’ve earned <b>10.00 tokens</b>! Keep sharing and earning more! 🚀",
			user.FirstName, user.Id), &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})

		err = updateUserBalance(referrerID, 10.0)
		if err != nil {
			log.Printf("Failed to update referrer's balance: %v", err)
		}
	}

	// Register the user (if no referrer)
	if referrerID == 0 {
		err = addUser(User{
			ID:       user.Id,
			Balance:  0,
			Referrer: 0,
		})

		if err != nil {
			log.Printf("Failed to add user: %v", err)
			_, _ = msg.Reply(b, "❌ <b>Failed to register. Please try again later.</b>", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})

			return nil
		}
	}

	// Success message for the new user
	response := fmt.Sprintf(
		"🎉 <b>Welcome to the Refer & Earn Bot, %s!</b>\n\n"+
			"💰 <b>Balance:</b> %.2f\n"+
			"🤝 <b>Referred Users:</b> %d\n\n"+
			"🔗 Use your referral link to invite friends and earn rewards!",
		user.FirstName, 0.00, 0)

	_, _ = msg.Reply(b, response, &gotgbot.SendMessageOpts{
		ReplyMarkup: button,
		ParseMode:   "HTML",
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
	})

	return nil
}
func help(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	text := `
<b>🤖 Bot Commands</b>
Here are the commands you can use:

<b>🔹 General Commands</b>
/start - 🚀 Start the bot  
/help - 📖 Show this help message  
/info - ℹ️ Show your user info  
/accno - 🆔 Set or update account number 

<b>🔸 Owner Commands</b>
/add - ➕ Add balance  
/remove - ➖ Remove balance  
/stats - 📊 Show bot statistics  
/broadcast - 📢 Broadcast a message to all users  

⚠️ <i>Note: Owner commands are restricted to the bot owner only.</i>
`

	button := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         " Home",
					CallbackData: "home",
				},
			},
		},
	}
	_, _ = msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: button,
	})

	return nil
}

func info(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]
	var userId int64

	if len(args) == 0 {
		userId = user.Id
	} else {
		userId = stringToInt64(args[0])
	}

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ <b>User not found.</b>\n\nPlease check the User ID and try again.", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	response := fmt.Sprintf(
		"👤 <b>User Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💰 <b>Account Balance:</b> %.2f\n"+
			"<b>Account Number</b> %d",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance, userInfo.AccNo)

	_, _ = msg.Reply(b, response, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})

	return nil
}

func infoCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data
	splitData := strings.Split(callbackData, ".")
	if len(splitData) < 2 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid callback data.",
			ShowAlert: true,
		})
		return nil
	}

	userId := stringToInt64(splitData[1])

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ User not found.",
			ShowAlert: true,
		})

		_, _, _ = msg.EditText(b, "❌ <b>User not found.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         " Home",
					CallbackData: "home",
				},
			},
		},
	}
	response := fmt.Sprintf(
		"👤 <b>User Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💰 <b>Account Balance:</b> %.2f\n"+
			"<b>Account Number</b> %d",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance, userInfo.AccNo)

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "ℹ️ User information loaded successfully.",
	})

	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: button,
	})

	return nil
}
func walletCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data

	splitData := strings.Split(callbackData, ".")
	if len(splitData) < 2 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid callback data.",
			ShowAlert: true,
		})
		return nil
	}
	userId := stringToInt64(splitData[1])
	userInfo, err := getUser(userId)

	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ User not found.",
			ShowAlert: true,
		})

		_, _, _ = msg.EditText(b, "❌ <b>User not found.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})

		return nil
	}

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "🆔 Set Account Number",
					CallbackData: fmt.Sprintf("setAccNo.%d", userInfo.ID),
				},
			},
			{
				{
					Text:         "💸 Withdraw",
					CallbackData: fmt.Sprintf("withdraw.%d", userInfo.ID),
				},
				{
					Text:         " Home",
					CallbackData: "home",
				},
			},
		},
	}

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "Wallet information loaded.",
	})

	response := fmt.Sprintf(
		"💰 <b>Wallet Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💵 <b>Account Balance:</b> %.2f",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)

	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ReplyMarkup: button,
		ParseMode:   "HTML",
	})

	return nil
}

func addBalance(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "❌ You are not authorized to use this command.", nil)
		return nil
	}

	args := ctx.Args()[1:]
	if len(args) < 2 {
		_, _ = msg.Reply(b, "❌ Invalid arguments.\n\nUsage: <code>/add &lt;user_id&gt; &lt;amount&gt;</code>", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	userId := stringToInt64(args[0])
	if userId <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid user ID. Please enter a valid numeric user ID.", nil)
		return nil
	}

	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil || amount <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid amount. Please enter a positive number.", nil)
		return nil
	}

	err = updateUserBalance(userId, amount)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update balance: %v", err), nil)
		return nil
	}

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	text := fmt.Sprintf(
		"✅ Successfully updated balance for user <b>%d</b>.\n\n"+
			"🔹 <b>Amount Added:</b> %.2f\n"+
			"💵 <b>New Balance:</b> %.2f",
		userId, amount, userInfo.Balance,
	)
	_, _ = msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})

	return nil
}

func removeBalanceCmd(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser

	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "❌ You are not authorized to use this command.", nil)
		return nil
	}

	args := ctx.Args()[1:]
	if len(args) < 2 {
		_, _ = msg.Reply(b, "❌ Invalid arguments.\n\nUsage: <code>/remove &lt;user_id&gt; &lt;amount&gt;</code>", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	userId := stringToInt64(args[0])
	if userId <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid user ID. Please enter a valid numeric user ID.", nil)
		return nil
	}

	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil || amount <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid amount. Please enter a positive number.", nil)
		return nil
	}

	_, err = removeBalance(userId, amount)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update balance: %v", err), nil)
		return nil
	}

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	text := fmt.Sprintf(
		"✅ Successfully updated balance for user <b>%d</b>.\n\n"+
			"🔹 <b>Amount Deducted:</b> %.2f\n"+
			"💵 <b>New Balance:</b> %.2f",
		userId, amount, userInfo.Balance,
	)
	_, _ = msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})

	return nil
}

func updateAccNo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]

	if len(args) < 1 {
		_, _ = msg.Reply(b, "❌ Please provide an account number.\n\nUsage: /accno <account_number>", nil)
		return nil
	}

	accNo := stringToInt64(args[0])
	if accNo <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid account number. Please enter a valid positive number.", nil)
		return nil
	}

	err := updateUserAccNo(user.Id, accNo)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update account number: %v", err), nil)
		return nil
	}

	userInfo, err := getUser(user.Id)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	text := fmt.Sprintf(
		"✅ Account number successfully updated for user <b>%d</b>.\n\n"+
			"🔹 <b>New Account Number:</b> %d",
		user.Id, userInfo.AccNo,
	)
	_, _ = msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})

	return nil
}

func stats(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "You are not authorized to use this command.", nil)
		return nil
	}

	allUser, _ := getAllUsers()
	text := fmt.Sprintf("Total Users: %d\n\n", len(allUser))
	_, _ = msg.Reply(b, text, nil)
	return nil
}

func broadcast(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if msg.Chat.Type != "private" {
		return nil
	}

	if msg.From.Id != OwnerID {
		_, _ = msg.Reply(b, "You must be the owner to use this command.", nil)
		return nil
	}

	reply := ctx.EffectiveMessage.ReplyToMessage
	if reply == nil {
		_, err := ctx.EffectiveMessage.Reply(b, "❌ <b>Reply to a message to broadcast</b>", &gotgbot.SendMessageOpts{ParseMode: "HTML"})
		if err != nil {
			return fmt.Errorf("error while replying to user: %v", err)
		}
		return ext.EndGroups
	}

	button := &gotgbot.InlineKeyboardMarkup{}
	if reply.ReplyMarkup != nil {
		button.InlineKeyboard = reply.ReplyMarkup.InlineKeyboard
	}

	users, err := getAllUsers()
	if err != nil {
		_, _ = msg.Reply(b, "Error getting users.\n\n"+CustomError(err).Error(), nil)
		return err
	}

	successfulBroadcasts := 0
	for _, u := range users {
		user_id := u.ID
		_, err = b.CopyMessage(user_id, ctx.EffectiveMessage.Chat.Id, reply.MessageId, &gotgbot.CopyMessageOpts{ReplyMarkup: button})

		if err == nil {
			successfulBroadcasts++
		}

		time.Sleep(33 * time.Millisecond)
	}

	_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("✅ <b>Broadcast successfully to %d users</b>", successfulBroadcasts), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	if err != nil {
		return err
	}

	return nil
}

func cancel(b *gotgbot.Bot, ctx *ext.Context) error {
	button := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         " Home",
					CallbackData: "home",
				},
			},
		},
	}

	_, err := ctx.EffectiveMessage.Reply(b, "❌ <b>Conversation cancelled</b>", &gotgbot.SendMessageOpts{
		ParseMode:   "html",
		ReplyMarkup: button,
	})

	if err != nil {
		return fmt.Errorf("failed to send cancel message: %w", err)
	}

	return handlers.EndConversation()
}

func setAccNo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	query := ctx.CallbackQuery

	_, err := getUser(user.Id)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ User not found.",
			ShowAlert: true,
		})
		_, _, _ = msg.EditText(b, "❌ <b>User not found.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "✅ To set the account number, please enter the account number.",
		ShowAlert: true,
	})

	_, _, err = msg.EditText(b, "✅ To set the account number, please enter the account number.\nTo cancel, click /cancel .", &gotgbot.EditMessageTextOpts{
		ParseMode: "html",
	})

	if err != nil {
		log.Printf("Error while editing message: %v", err)
		_, _ = msg.Reply(b, "❌ Something went wrong while processing your request. Please try again.", nil)
		return handlers.EndConversation()
	}

	return handlers.NextConversationState(SET_ACC)
}

func setAccAsk(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser

	_, err := getUser(user.Id)
	if err != nil {
		_, _ = msg.Reply(b, "❌ User not found.", nil)
		return nil
	}

	accNoInt64 := stringToInt64(msg.Text)
	if accNoInt64 <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid account number. Please enter a valid positive number.", nil)
		return nil
	}

	err = updateUserAccNo(user.Id, accNoInt64)
	if err != nil {
		log.Printf("Error while setting account number for user %d: %v", user.Id, err)
		_, _ = msg.Reply(b, "❌ Something went wrong while processing your request. Please try again.", nil)
		return handlers.EndConversation()
	}

	_, _ = msg.Reply(b, "✅ Account number set successfully.", nil)
	return handlers.EndConversation()
}

func withdrawal(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.Update.CallbackQuery
	user := ctx.EffectiveUser

	userInfo, err := getUser(user.Id)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ User not found.",
			ShowAlert: true,
		})
		_, _, _ = msg.EditText(b, "❌ <b>User not found.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	if userInfo.Balance <= 0 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ You have no balance to withdraw.",
			ShowAlert: true,
		})
		_, _, _ = msg.EditText(b, "❌ <b>You have no balance to withdraw.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	if userInfo.AccNo == 0 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ You have no account number to withdraw to.",
			ShowAlert: true,
		})
		_, _, _ = msg.EditText(b, "❌ <b>You have no account number to withdraw to.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "💸 Please enter the amount you'd like to withdraw.",
		ShowAlert: true,
	})

	_, _, err = msg.EditText(b, "💸 Please send the amount you wish to withdraw.\nFor cancel use /cancel", &gotgbot.EditMessageTextOpts{
		ParseMode: "html",
	})

	if err != nil {
		log.Printf("❌ Error while editing message: %v", err)
		_, _ = msg.Reply(b, "❌ Something went wrong while processing your request. Please try again.", nil)
		return handlers.EndConversation()
	}

	// Proceed to the next conversation state
	return handlers.NextConversationState(WITHDWAWL)
}

func withdrawalAsk(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	text := msg.GetText()

	// Parse the withdrawal amount
	amount, err := strconv.ParseFloat(text, 64)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Oops! Invalid amount. Please enter a valid number to withdraw. 💸", nil)
		return handlers.NextConversationState(WITHDWAWL)
	}

	// Get user data
	userInfo, err := getUser(user.Id)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Something went wrong. "+CustomError(err).Error()+" Please try again later.", nil)
		return handlers.EndConversation()
	}

	// Check if the user has sufficient balance
	if amount > userInfo.Balance {
		_, _ = msg.Reply(b, "❌ Insufficient balance. 💳 Please try again with a valid amount.", nil)
		return handlers.NextConversationState(WITHDWAWL)
	}

	// Remove balance from user account
	_, err = removeBalance(msg.From.Id, amount)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to process your withdrawal request. "+err.Error(), nil)
		return handlers.EndConversation()
	}

	// Send confirmation button
	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "✅ Confirm Withdrawal",
					CallbackData: fmt.Sprintf("confirm_withdrawal.%d.%f", user.Id, amount),
				},
			},
		},
	}

	// Log the withdrawal request
	loggerMsg := fmt.Sprintf("💰 <b>%s</b> requested a withdrawal of %.2f\n\nUser AccNo: <code>%d</code>", user.FirstName, amount, userInfo.AccNo)

	// Send to logger
	_, err = b.SendMessage(LoggerID, loggerMsg, &gotgbot.SendMessageOpts{ReplyMarkup: button, ParseMode: "html"})
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to send withdrawal request to the logger. "+CustomError(err).Error(), nil)
		return handlers.EndConversation()
	}

	_, _ = msg.Reply(b, "🎉 Withdrawal Request Submitted! 🎉\n\n- 🕒 Processing Time: Please allow a few hours for our team to review and approve your request.", nil)

	return handlers.EndConversation()
}

func confirmWithdrawal(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.Update.CallbackQuery
	data := query.Data

	splitData := strings.Split(data, ".")
	if len(splitData) < 3 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid callback data.",
			ShowAlert: true,
		})
		return nil
	}

	userID, err := strconv.ParseInt(splitData[1], 10, 64)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid user ID.",
			ShowAlert: true,
		})
		return nil
	}

	amount, err := strconv.ParseFloat(splitData[2], 64)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid amount.",
			ShowAlert: true,
		})
		return nil
	}

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "✅ Processing withdrawal request...",
	})

	_, _, _ = msg.EditText(b, fmt.Sprintf("✅ Done! Amount of %.2f successfully withdrawn.", amount), nil)

	text := fmt.Sprintf(`🎉 Withdrawal Approved! 🎉

✅ Your withdrawal request has been successfully approved!

💸 Amount: %.2f

Thank you for trusting us! 🚀`, amount)

	_, err = b.SendMessage(userID, text, nil)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to send the approved withdrawal message. "+CustomError(err).Error(), nil)
	}

	return nil
}

func home(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	quary := ctx.CallbackQuery

	referUrl := fmt.Sprintf("https://t.me/%s?start=%d", b.User.Username, user.Id)

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: "👤 Owner",
					Url:  fmt.Sprintf("tg://user?id=%d", OwnerID),
				},
			},
			{
				{
					Text: "🔗 Refer & Earn",
					Url:  fmt.Sprintf("https://t.me/share/url?url=%s", referUrl),
				},
				{
					Text:         "ℹ️ Info",
					CallbackData: fmt.Sprintf("info.%d", user.Id),
				},
			},
			{
				{
					Text:         "💼 Wallet",
					CallbackData: fmt.Sprintf("wallet.%d", user.Id),
				},
				{
					Text:         "💸 Withdraw",
					CallbackData: fmt.Sprintf("withdraw.%d", user.Id),
				},
			},
		},
	}
	_, _ = quary.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "🔙 Back to Main Menu",
	})

	existingUser, _ := getUser(user.Id)
	response := fmt.Sprintf(
		"👋 <b>Welcome back, %s!</b>\n\n"+
			"💰 <b>Balance:</b> %.2f\n"+
			"🤝 <b>Referred Users:</b> %d\n\n"+
			"🚀 Keep earning rewards by referring your friends!",
		user.FirstName, existingUser.Balance, len(existingUser.ReferredUsers))

	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: button,
	})

	return nil
}
