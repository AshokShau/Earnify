# Earnify - Telegram Refer & Earn Bot

**Earnify** is a Telegram bot that allows users to refer their friends and earn rewards. It's a referral and earning system where users can get rewarded by referring others to the bot.

---

## Features

- **Referral System**: Users can refer their friends using a unique referral code and earn rewards.
- **Balance Management**: Users can check their balance, earn tokens, and redeem rewards.
- **Admin Panel**: Admins can manage balances, stats, and broadcast messages.
- **User Information**: Users can view their account details, including referred users and account balance.
- **Wallet System**: Users can withdraw their rewards through the wallet system.
- **Account Number Management**: Users can set or update their account number.
- **Statistics**: Admins can view bot statistics.
- **Broadcast Messages**: Admins can broadcast messages to all users.
- **Force Subscription**: Users can be forced to subscribe to the bot's channel.
---

## Commands

### For Users:

- `/start` - Start the bot and get your referral link.
- `/help` - Show a list of available commands.
- `/info` - Show your user info, including balance and referred users.
- `/wallet` - Check your current balance and access withdrawal options.
- `/accno <account_number>` - Set or update a user's account number.

### For Admins (Owner Only):

- `/add <user_id> <amount>` - Add balance to a user's account.
- `/remove <user_id> <amount>` - Remove balance from a user's account.
- `/stats` - View bot statistics like total users, total rewards, etc.
- `/broadcast` - Send a message to all users.

---

## Bot Setup

### Prerequisites:
- **Go** (version letest one)
- **MongoDB** for storing user data and balance.
- **Telegram Bot Token**: You need to create a bot on Telegram and get a bot token from [BotFather](https://core.telegram.org/bots#botfather).

### Installation:

#### Docker installation: (Recommended)

1. Clone the repository: `git clone https://github.com/AshokShau/earnify.git && cd earnify`
2. Set up the environment: `cp sample.env .env`
3. Build the Docker image: `docker build -t earnify .`
4. Run the Docker container: `docker run -p 8080:8080 -d earnify`

---

#### Manual installation:

1. Clone the repository: `git clone https://github.com/AshokShau/earnify.git && cd earnify`
2. Set up the environment: `cp sample.env .env`
3. Build the project: `go build`
4. Run the bot: `./earnify`

---

## Configuration

- **MongoDB**: The bot uses MongoDB to store user data, including their balance, referral links, and referred users. Make sure your MongoDB instance is running.
- **Bot Token**: You need to create a bot on Telegram through BotFather and provide the bot token in your environment variables.
- **Owner ID**: Set your Telegram user ID as the owner in the environment variables for administrative commands.
- **Logger ID**: Set your Telegram user ID as the logger in the environment variables for logging messages.

---

## Contributing

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature`).
3. Commit your changes (`git commit -am 'Add new feature'`).
4. Push to the branch (`git push origin feature/your-feature`).
5. Open a pull request.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Contact

- [GitHub](https://github.com/AshokShau/earnify)
- [Telegram](https://t.me/AshokShau)

---

## 📝 Note

This repository, **Earnify**, was born out of pure boredom and a sprinkle of curiosity. While it serves its purpose as a functional bot to manage earnings and more, future updates are highly dependent on my mood. 😴 

That means updates might roll out when inspiration strikes—or maybe not at all. It’s a bit like a gamble, but hey, isn’t life the same? 🎲

Feel free to fork, contribute, or just enjoy it as-is. Happy earning! 💸
