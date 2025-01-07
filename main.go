package main

import (
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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	_ "github.com/joho/godotenv/autoload"
)

var secretToken = "efjernfernferfjfjn32nro"
var allowedUpdates = []string{"message", "callback_query"}

var (
	WebhookURL string
	Port       string
	MongoDBURI string
	OwnerID    int64
)

func main() {
	var err error
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN is not set")
	}
	WebhookURL = os.Getenv("WEBHOOK_URL")
	Port = os.Getenv("PORT")
	MongoDBURI = os.Getenv("MONGO_URI")

	OwnerID, err = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if err != nil {
		log.Fatal("OWNER_ID is not set")
	}

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
		log.Println("Starting polling...")
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

	referUrl := fmt.Sprintf("https://t.me/%s?start=%d", b.User.Username, user.Id)

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: "👤 Owner",
					Url:  "https://t.me/MaybeRadhaa",
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
			},
		},
	}

	// Check if the user exists in the database
	existingUser, err := getUser(user.Id)
	if err != nil && err.Error() != "mongo: no documents in result" {
		log.Printf("Failed to fetch user: %v", err)
		_, _ = msg.Reply(b, "❌ An error occurred. Please try again later.\n/start", nil)
		return nil
	}

	if existingUser != nil {
		// Existing user message
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

	// If new user, check for referral code
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

		// Check if the referrer exists
		referrer, err := getUser(referrerID)
		if err != nil {
			_, _ = msg.Reply(b, "❌ <b>The referral code is not valid.</b>\n\nPlease check with the person who referred you.", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})
			return nil
		}

		log.Printf("Referrer ID: %d", referrer.ID)

		// Register the user with the referrer
		err = referUser(referrerID, user.Id)
		if err != nil {
			log.Printf("Failed to refer user: %v", err)
			_, _ = msg.Reply(b, "⚠️ <b>Failed to register with the referral. Please try again.</b>", &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})
			return nil
		}

		// Notify referrer about a successful referral
		_, _ = b.SendMessage(referrerID, fmt.Sprintf(
			"🎉 <b>Referral Successful!</b>\n\n"+
				"👤 You referred <b>%s</b> (%d) successfully!\n"+
				"💵 You’ve earned <b>10.00 DOGS tokens</b>! Keep sharing and earning more! 🚀",
			user.FirstName, user.Id), &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})

		// Update referrer's balance
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
	_, _ = msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})
	return nil
}
func info(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]
	var userId int64

	// Determine user ID (self or provided as argument)
	if len(args) == 0 {
		userId = user.Id
	} else {
		userId = stringToInt64(args[0])
	}

	// Fetch user information from the database
	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ <b>User not found.</b>\n\nPlease check the User ID and try again.", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	// Format the response with improved text and emojis
	response := fmt.Sprintf(
		"👤 <b>User Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💰 <b>Account Balance:</b> %.2f\n",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)

	// Send the response
	_, _ = msg.Reply(b, response, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	})
	return nil
}

func infoCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data

	// Split and parse callback data
	splitData := strings.Split(callbackData, ".")
	if len(splitData) < 2 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid callback data.",
			ShowAlert: true,
		})
		return nil
	}
	userId := stringToInt64(splitData[1])

	// Fetch user info
	userInfo, err := getUser(userId)
	if err != nil {
		// Respond to callback query with an error
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ User not found.",
			ShowAlert: true,
		})
		_, _, _ = msg.EditText(b, "❌ <b>User not found.</b>", &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	// Format response with rich text and emojis
	response := fmt.Sprintf(
		"👤 <b>User Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💰 <b>Account Balance:</b> %.2f",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)

	// Answer the callback query with success message
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "ℹ️ User information loaded successfully.",
	})

	// Edit message with user information
	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ParseMode: "HTML",
	})

	return nil
}
func walletCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data

	// Parse callback data
	splitData := strings.Split(callbackData, ".")
	if len(splitData) < 2 {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "❌ Invalid callback data.",
			ShowAlert: true,
		})
		return nil
	}
	userId := stringToInt64(splitData[1])

	// Fetch user information
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

	// Prepare inline keyboard with withdrawal option
	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "💸 Withdraw Dogs Token",
					CallbackData: fmt.Sprintf("withdraw.%d", userInfo.ID),
				},
			},
		},
	}

	// Create a user-friendly response
	response := fmt.Sprintf(
		"💰 <b>Wallet Information</b>\n\n"+
			"🔹 <b>User ID:</b> %d\n"+
			"🔗 <b>Referrer ID:</b> %d\n"+
			"🤝 <b>Referred Users:</b> %d\n"+
			"💵 <b>Account Balance:</b> %.2f",
		userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)

	// Edit the message with wallet information and button
	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ReplyMarkup: button,
		ParseMode:   "HTML",
	})

	// Respond to callback query
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "ℹ️ Wallet information loaded.",
	})
	return nil
}

func addBalance(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser

	// Authorization check
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "❌ You are not authorized to use this command.", nil)
		return nil
	}

	// Validate arguments
	args := ctx.Args()[1:]
	if len(args) < 2 {
		_, _ = msg.Reply(b, "❌ Invalid arguments.\n\nUsage: <code>/add &lt;user_id&gt; &lt;amount&gt;</code>", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	// Parse user ID
	userId := stringToInt64(args[0])
	if userId <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid user ID. Please enter a valid numeric user ID.", nil)
		return nil
	}

	// Parse amount
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil || amount <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid amount. Please enter a positive number.", nil)
		return nil
	}

	// Add balance to the user
	err = updateUserBalance(userId, amount)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update balance: %v", err), nil)
		return nil
	}

	// Fetch updated user info
	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	// Success message
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

	// Ensure only the owner can execute the command
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "❌ You are not authorized to use this command.", nil)
		return nil
	}

	// Validate arguments
	args := ctx.Args()[1:]
	if len(args) < 2 {
		_, _ = msg.Reply(b, "❌ Invalid arguments.\n\nUsage: <code>/remove &lt;user_id&gt; &lt;amount&gt;</code>", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		return nil
	}

	// Parse user ID
	userId := stringToInt64(args[0])
	if userId <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid user ID. Please enter a valid numeric user ID.", nil)
		return nil
	}

	// Parse amount
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil || amount <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid amount. Please enter a positive number.", nil)
		return nil
	}

	// Remove balance
	_, err = removeBalance(userId, amount)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update balance: %v", err), nil)
		return nil
	}

	// Fetch updated user info
	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	// Success message
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

	// Check if the user provided an account number
	if len(args) < 1 {
		_, _ = msg.Reply(b, "❌ Please provide an account number.\n\nUsage: /accno <account_number>", nil)
		return nil
	}

	// Validate the account number (should be a valid integer)
	accNo, err := stringToInt64(args[0])
	if err != nil || accNo <= 0 {
		_, _ = msg.Reply(b, "❌ Invalid account number. Please enter a valid positive number.", nil)
		return nil
	}

	// Update the account number in the database
	err = updateUserAccNo(user.Id, accNo)
	if err != nil {
		_, _ = msg.Reply(b, fmt.Sprintf("❌ Failed to update account number: %v", err), nil)
		return nil
	}

	// Retrieve the updated user info
	userInfo, err := getUser(user.Id)
	if err != nil {
		_, _ = msg.Reply(b, "❌ Failed to retrieve updated user information.", nil)
		return nil
	}

	// Success message with updated account number
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
		_, _ = msg.Reply(b, "Error getting users.\n\n"+err.Error(), nil)
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
