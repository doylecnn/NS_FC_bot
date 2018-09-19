package main

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"log"

	"github.com/doylecnn/NSFCbot/command"
	"gopkg.in/telegram-bot-api.v4"
)

// AddFC command
type AddFC struct{ db *sql.DB }

// Do command
func (c AddFC) Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	if len(message.Text) <= 7 {
		return nil, errors.New("message too short")
	}

	idx := strings.IndexByte(message.Text, ' ')
	if idx < -1 {
		return nil, fmt.Errorf("command AddFC not contain ' '")
	}
	msg := strings.TrimSpace(message.Text[idx:])
	var cmdAddFC = regexp.MustCompile("^(?:[sS][wW]-)?((?:\\d{12})|(?:\\d{4}-\\d{4}-\\d{4}))$")
	submatch := cmdAddFC.FindAllStringSubmatch(msg, 1)

	if len(submatch) != 1 {
		return nil, command.Error{InnerError: fmt.Errorf("the friend code format is unacceptable. message is: %s", msg),
			ReplyText: "FC 格式错，接受完整FC 格式或不含 - 或 SW 的12位纯数字。"}
	}
	fc, _ := strconv.ParseInt(strings.Replace(submatch[0][1], "-", "", -1), 10, 64)

	trans, err := c.db.Begin()
	if err != nil {
		return nil, command.Error{InnerError: err, ReplyText: "出错了，请重试。"}
	}
	username := message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName + " " + message.From.LastName
	}
	var exists = 0
	err = trans.QueryRow("SELECT count(1) FROM user_NSFC WHERE userid = :userid and fc = :fc", sql.Named("userid", message.From.ID), sql.Named("fc", fc)).Scan(&exists)
	if exists != 0{
		return nil, command.Error{InnerError: err, ReplyText: "您已在其它群登记过FC了，本次操作将永久允许本群查询到您的FC。"}
	}
	_, err = trans.Exec("INSERT INTO user_NSFC(userid, fc, username) VALUES(:userid, :fc, :username) ON CONFLICT(userid) DO UPDATE SET fc = :fc, username = :username where userid = :userid",
		sql.Named("userid", message.From.ID),
		sql.Named("fc", fc),
		sql.Named("username", username))
	if err != nil {
		trans.Rollback()
		return nil, command.Error{InnerError: err, ReplyText: "出错了，请重试。"}
	}
	_, err = trans.Exec("INSERT INTO group_user(groupID, userID) VALUES(:groupID, :userID)", sql.Named("groupID", message.Chat.ID), sql.Named("userID", message.From.ID))
	trans.Commit()
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			Text: "完成。"},
		nil
}

// FC command
type FC struct{ db *sql.DB }

// Do FC command
func (c FC) Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	if message.ReplyToMessage == nil {
		return nil, errors.New("not reply to any message")
	}

	replyToUserID := message.ReplyToMessage.From.ID
	row := c.db.QueryRow("select userid, fc, username from user_NSFC where userid = :userid", sql.Named("userid", replyToUserID))
	var userid, username string
	var fc int64
	err = row.Scan(&userid, &fc, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, command.Error{InnerError: err, ReplyText: "他/她还没有登记过FC。"}
		}
		return nil, err
	}
	var exists = 0
	err = c.db.QueryRow("select count(1) from group_user where groupID = :groupID and userID = :userID", sql.Named("groupID",message.Chat.ID), sql.Named("userID",userid)).Scan(&exists)
	if err == sql.ErrNoRows || exists == 0{
		return nil, command.Error{InnerError: err, ReplyText: "他/她还未 在本群 登记过FC"}
	}
	if err != nil {
		return nil, err
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			ParseMode:tgbotapi.ModeMarkdown,
			Text: fmt.Sprintf("[%s](tg://user?id=%s): %s", username, userid, friendCodeFormat(fc))},
		nil
}

// MyFC command
type MyFC struct{ db *sql.DB }

// Do MyFC command
func (c MyFC) Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	row := c.db.QueryRow("select userid, fc, username from user_NSFC where userid = :userid", sql.Named("userid", message.From.ID))
	var userid, username string
	var fc int64
	err = row.Scan(&userid, &fc, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, command.Error{InnerError: err, ReplyText: "你还未登记过你的fc。\n请使用 /addfc 添加你的fc。"}
		}
		return nil, err
	}

	var exists = 0
	err = c.db.QueryRow("select count(1) from group_user where groupID = :groupID and userID = :userID", sql.Named("groupID",message.Chat.ID), sql.Named("userID",userid)).Scan(&exists)
	if err == sql.ErrNoRows || exists == 0{
		c.db.Exec("INSERT INTO group_user(groupID, userID) VALUES(:groupID, :userID)", sql.Named("groupID", message.Chat.ID), sql.Named("userID", message.From.ID))
		return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			ParseMode:tgbotapi.ModeMarkdown,
			Text: fmt.Sprintf("[%s](tg://user?id=%s): %s\n本次操作将永久允许本群查询到您的FC。", username, userid, friendCodeFormat(fc))},
		nil
	}
	if err != nil {
		return nil, err
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			ParseMode:tgbotapi.ModeMarkdown,
			Text: fmt.Sprintf("[%s](tg://user?id=%s): %s", username, userid, friendCodeFormat(fc))},
		nil
}

// SFC command
type SFC struct{ db *sql.DB }

// Do SFC command
func (c SFC) Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	if len(args) <= 1 {
		return
	}
	row := c.db.QueryRow("select userid, fc, username from user_NSFC where username = :username", sql.Named("username",args[1:]))
	var userid, username string
	var fc int64
	err = row.Scan(&userid, &fc, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, command.Error{InnerError: err, ReplyText: "他/她还未登记过FC"}
		}
		return nil, err
	}
	var exists = 0
	err = c.db.QueryRow("select count(1) from group_user where groupID = :groupID and userID = :userID", sql.Named("groupID",message.Chat.ID), sql.Named("userID",userid)).Scan(&exists)
	if err == sql.ErrNoRows || exists == 0{
		return nil, command.Error{InnerError: err, ReplyText: "他/她还未 在本群 登记过FC"}
	}
	if err != nil {
		return nil, err
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			ParseMode:tgbotapi.ModeMarkdown,
			Text: fmt.Sprintf("[%s](tg://user?id=%s): %s", username, userid, friendCodeFormat(fc))},
		nil
}

// FCList command
type FCList struct{db *sql.DB}

// Do FCList command
func (c FCList) Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	rows, err := c.db.Query("SELECT a.userid, fc, username FROM user_NSFC a JOIN group_user b ON a.userid = b.userid WHERE groupID = :groupID", sql.Named("groupID",message.Chat.ID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var reply = []string{}
		for rows.Next() {
			var userid, username string
			var fc int64
			err := rows.Scan(&userid, &fc, &username)
			if err != nil {
				log.Println(err)
				continue
			}
			reply = append(reply, fmt.Sprintf("[%s](tg://user?id=%s): %s", username, userid, friendCodeFormat(fc)))
		}
		replys := strings.Join(reply, "\n")
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
				DisableNotification:true},
			ParseMode:tgbotapi.ModeMarkdown,
			Text: replys},
		nil
}

func friendCodeFormat(fc int64) string {
	return fmt.Sprintf("SW-%04d-%04d-%04d", fc/100000000%10000, fc/10000%10000, fc%10000)
}


// inline
func inlineQueryAnswer(db *sql.DB, query *tgbotapi.InlineQuery) (answer string, err error) {
	row := db.QueryRow("select * from user_NSFC where userid = :userid", sql.Named("userid", query.From.ID))
	var userid, username string
	var fc int64
	err = row.Scan(&userid, &fc, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			return "你还未登记你的fc。\n请使用 /addfc 添加你的fc。", err
		}
		return "", err
	}
	return fmt.Sprintf("[%s](tg://user?id=%s): %s", username, userid, friendCodeFormat(fc)),nil
}