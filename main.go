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

// Start function to handle /start command
func start(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]

	referUrl := fmt.Sprintf("https://t.me/%s?start=%d", b.User.Username, user.Id)

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: "Owner",
					Url:  "https://t.me/MaybeRadhaa",
				},
			},
			{
				{
					Text: "Refer & Earn",
					Url:  fmt.Sprintf("https://t.me/share/url?url=%s", referUrl),
				},
				{
					Text:         "Info",
					CallbackData: fmt.Sprintf("info.%d", user.Id),
				},
			},
			{
				{
					Text:         "Wallet",
					CallbackData: fmt.Sprintf("wallet.%d", user.Id),
				},
			},
		},
	}

	// Check if the user exists in the database
	existingUser, err := getUser(user.Id)
	if err != nil && err.Error() != "mongo: no documents in result" {
		log.Printf("Failed to fetch user: %v", err)
		_, _ = msg.Reply(b, "An error occurred. Please try again later.\n/start", nil)
		return nil
	}

	if existingUser != nil {
		// Existing user message
		response := fmt.Sprintf("<b>Welcome to the Refer & Earn Bot! 🎉</b>, %s!\nYour account balance: %.2f\nReferred users: %d\nEarn dogs token by referring friends and redeem reward", user.FirstName, existingUser.Balance, len(existingUser.ReferredUsers))

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
			_, _ = msg.Reply(b, "Invalid referral code. Please try again.", nil)
			return nil
		}

		// Check if the referrer exists
		referrer, err := getUser(referrerID)
		if err != nil {
			_, _ = msg.Reply(b, "The referral code is not valid.", nil)
			return nil
		}

		log.Printf("referrer ID: %d", referrer.ID)

		// Register the user with the referrer
		err = referUser(referrerID, user.Id)
		if err != nil {
			log.Printf("Failed to refer user: %v", err)
			url := fmt.Sprintf("https://t.me/%s?start=%d", b.User.Username, referrerID)
			_, _ = msg.Reply(b, "Failed to register with the referral. Please try again.\n"+url, nil)
			return nil
		}

		// Notify referrer about a successful referral
		_, _ = b.SendMessage(referrerID, fmt.Sprintf("You referred %s (%d) successfully!\nYou have been rewarded 10.00 dogs tokens", user.FirstName, user.Id), nil)

		// update referrer's balance
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
			_, _ = msg.Reply(b, "Failed to register. Please try again.\n", nil)
			return nil
		}
	}

	// Success message for the new user
	response := fmt.Sprintf("<b>Welcome to the Refer & Earn Bot! 🎉</b>, %s!\nYour account balance: %.2f\nReferred users: %d\nEarn dogs token by referring friends and redeem reward", user.FirstName, 0.00, 0)
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
/start - Start the bot
/help - Show this message
/info - Show user info
/add - Add balance
/accno - Set/update account number
/remove - Remove balance
`
	_, _ = msg.Reply(b, text, nil)
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
		_, _ = msg.Reply(b, "User not found.", nil)
		return nil
	}

	response := fmt.Sprintf("User ID: %d\nReferrer: %d\nReferred Users: %v\nAccount Balance: %.2f", userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)
	_, _ = msg.Reply(b, response, nil)
	return nil
}

func infoCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data
	splitData := strings.Split(callbackData, ".")
	userId := stringToInt64(splitData[1])

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "User not found."})

		_, _, _ = msg.EditText(b, "User not found.", nil)
		return nil
	}

	response := fmt.Sprintf("User ID: %d\nReferrer: %d\nReferred Users: %v\nAccount Balance: %.2f", userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)
	_, _, _ = msg.EditText(b, response, nil)
	return nil
}

func walletCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	query := ctx.CallbackQuery
	callbackData := query.Data
	splitData := strings.Split(callbackData, ".")
	userId := stringToInt64(splitData[1])

	userInfo, err := getUser(userId)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "User not found."})

		_, _, _ = msg.EditText(b, "User not found.", nil)
		return nil
	}

	button := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "Withdraw Dogs Token",
					CallbackData: fmt.Sprintf("withdraw.%d", userInfo.ID),
				},
			},
		},
	}

	response := fmt.Sprintf("User ID: %d\nReferrer: %d\nReferred Users: %d\nAccount Balance: %.2f", userInfo.ID, userInfo.Referrer, len(userInfo.ReferredUsers), userInfo.Balance)
	_, _, _ = msg.EditText(b, response, &gotgbot.EditMessageTextOpts{
		ReplyMarkup: button,
	})
	return nil
}

func addBalance(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "You are not authorized to use this command.", nil)
		return nil
	}

	args := ctx.Args()[1:]

	if len(args) < 2 {
		_, _ = msg.Reply(b, "Please provide user ID and amount.\n\nUsage: /add <user_id> 69", nil)
		return nil
	}

	userId := stringToInt64(args[0])
	amount := stringToInt64(args[1])

	err := updateUserBalance(userId, float64(amount))
	if err != nil {
		_, _ = msg.Reply(b, err.Error(), nil)
		return nil
	}

	userInfo, _ := getUser(userId)
	text := fmt.Sprintf("Balance updated for user %d by %d.\n\nThe new balance is %.2f", userId, amount, userInfo.Balance)

	_, _ = msg.Reply(b, text, nil)
	return nil
}

func removeBalanceCmd(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	if user.Id != OwnerID {
		_, _ = msg.Reply(b, "You are not authorized to use this command.", nil)
		return nil
	}

	args := ctx.Args()[1:]

	if len(args) < 2 {
		_, _ = msg.Reply(b, "Please provide user ID and amount.\n\nUsage: /remove <user_id> 69", nil)
		return nil
	}

	userId := stringToInt64(args[0])
	amount := stringToInt64(args[1])

	_, err := removeBalance(userId, float64(amount))
	if err != nil {
		_, _ = msg.Reply(b, err.Error(), nil)
		return nil
	}

	userInfo, _ := getUser(userId)
	text := fmt.Sprintf("Balance updated for user %d by %d.\n\nThe new balance is %.2f", userId, amount, userInfo.Balance)

	_, _ = msg.Reply(b, text, nil)
	return nil
}

func updateAccNo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	args := ctx.Args()[1:]

	if len(args) < 1 {
		_, _ = msg.Reply(b, "Please provide account number.\n\nUsage: /accno 123456789", nil)
		return nil
	}

	accNo := stringToInt64(args[0])
	err := updateUserAccNo(user.Id, accNo)
	if err != nil {
		_, _ = msg.Reply(b, err.Error(), nil)
		return nil
	}

	userInfo, _ := getUser(user.Id)
	text := fmt.Sprintf("Account number updated for user %d by %d.\n\nThe new account number is %d", user.Id, accNo, userInfo.AccNo)

	_, _ = msg.Reply(b, text, nil)
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
