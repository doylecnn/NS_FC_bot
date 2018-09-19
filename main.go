package main

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"time"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"gopkg.in/telegram-bot-api.v4"

	_ "github.com/mattn/go-sqlite3"

	"github.com/doylecnn/NSFCbot/command"
)

func main() {
	var err error

	exefile, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exePath := filepath.Dir(exefile)

	var config tomlConfig
	if _, err = toml.DecodeFile(filepath.Join(exePath, "config.toml"), &config); err != nil {
		log.Fatalln(err)
		return
	}

	db, err := sql.Open("sqlite3", filepath.Join(exePath, config.Database.DBName))
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
	initDB(db)

	var (
		addFC = AddFC{db:db}
		myFC = MyFC{db:db}
		fc = FC{db:db}
		sfc = SFC{db:db}
		fclist = FCList{db:db}
		router = command.NewRouter()
	)
	router.HandleFunc("addfc", addFC.Do)
	router.HandleFunc("myfc", myFC.Do)
	router.HandleFunc("fc", fc.Do)
	router.HandleFunc("sfc", sfc.Do)
	router.HandleFunc("fclist", fclist.Do)

	bot := initBotAPI(config.Telegram.Token, config.Misc.Proxy)
	bot.Debug = config.Telegram.Debug
	log.Printf("Authorized on account: %s", bot.Self.UserName)
	log.Printf("dbname: %s", config.Database.DBName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = config.Telegram.UpdateTimeout

	updates, err := bot.GetUpdatesChan(u)
	time.Sleep(time.Millisecond * 500)
	updates.Clear()

	for update := range updates {
		if update.InlineQuery != nil {
			if update.InlineQuery.Query == "myFC"{
				text, err := inlineQueryAnswer(db, update.InlineQuery)
				if err != nil{
					log.Println(err)
					continue
				}
				article := tgbotapi.NewInlineQueryResultArticleMarkdown(update.InlineQuery.ID, "MyFC", text)
				article.Description = update.InlineQuery.Query
				btns := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("赞","赞的event_id")))
				article.ReplyMarkup = &btns

				inlineConf := tgbotapi.InlineConfig{
					InlineQueryID: update.InlineQuery.ID,
					IsPersonal:    true,
					CacheTime:     0,
					Results:       []interface{}{article},
				}

				if _, err := bot.AnswerInlineQuery(inlineConf); err != nil {
					log.Println(err)
				}
			}
		}else if update.CallbackQuery != nil{
			callback := update.CallbackQuery
			log.Println(callback.Data)
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID,"测试成功"))
		}else if update.Message != nil{
			go func(message *tgbotapi.Message){
				if (message.Chat.IsGroup() || message.Chat.IsSuperGroup()) && message.IsCommand() {
					messageSendTime := time.Unix(int64(message.Date), 0)
					if time.Since(messageSendTime).Seconds() > 30 {
						return
					}
					replyMessage, err := router.Run(message)
					if err != nil {
						log.Printf("%s", err.InnerError)
						if len(err.ReplyText) > 0 {
							replyMessage = &tgbotapi.MessageConfig{
								BaseChat: tgbotapi.BaseChat{
									ChatID:           message.Chat.ID,
									ReplyToMessageID: message.MessageID},
								Text: err.ReplyText}
						}
					}
					if replyMessage != nil {
						bot.Send(*replyMessage)
					}
				}
			}(update.Message)
		}
	}
}

func initDB(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_NSFC(userID int64 not null unique, fc int64 not null, username text not null);
CREATE UNIQUE INDEX IF NOT EXISTS nsfc_idx_1 ON user_NSFC (userID, fc);
CREATE TABLE IF NOT EXISTS group_user(groupID not null, userID int64 not null);
CREATE UNIQUE INDEX IF NOT EXISTS group_idx_1 ON group_user (groupID, userid);`)
	if err != nil {
		log.Fatalln(err)
	}
}

func initBotAPI(token, proxy string) (bot *tgbotapi.BotAPI) {
	var err error
	if len(proxy) > 0 {
		client, err := createProxyClient(proxy)
		if err != nil {
			log.Fatalln(err)
		}
		bot, err = tgbotapi.NewBotAPIWithClient(token, client)
		if err != nil {
			log.Fatalf("Some error occur: %s.\n", err)
		}
	}else{
		bot, err = tgbotapi.NewBotAPI(token)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return
}

func createProxyClient(proxy string) (client *http.Client, err error) {
	log.Println("verify proxy:", proxy)
	var proxyURL *url.URL
	proxyURL, err = url.Parse(proxy)
	if err == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
		var r *http.Response
		r, err = client.Get("https://www.google.com")
		if err != nil || r.StatusCode != 200 {
			return
		}
	}
	return
}