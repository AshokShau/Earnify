package main

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func stringToInt64(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func CustomError(err error) error {
	if err == nil {
		return nil
	}

	tokenRegex := regexp.MustCompile(`\d{9}:[A-Za-z0-9_-]{35}`)
	return errors.New(tokenRegex.ReplaceAllString(err.Error(), "$TOKEN"))
}

func onlyFloat64(msg *gotgbot.Message) bool {
	if msg.Text != "" {
		_, err := strconv.ParseFloat(msg.Text, 64)
		return err == nil
	} else {
		return false
	}
}

func onlyInt64(msg *gotgbot.Message) bool {
	if msg.Text != "" {
		_, err := strconv.ParseInt(msg.Text, 10, 64)
		return err == nil
	} else {
		return false
	}
}
