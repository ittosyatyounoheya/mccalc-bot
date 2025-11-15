package main

import (
	"fmt"
	"log"
	"math"
	"net/http" // ★ 追加
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// parseAmount は入力文字列からスタックサイズとアイテム総数を解析します。
// 例: "35000" -> 64, 35000
// 例: "1234@32" -> 32, 1234
func parseAmount(s string) (int, int, error) {
	sp := strings.Split(s, "@")
	if len(sp) == 2 {
		// @ の右側がスタックサイズ
		i2, err := strconv.Atoi(sp[1])
		if err != nil {
			return 64, 0, fmt.Errorf("strconv error: sp1 (%s)", sp[1])
		}
		// @ の左側がアイテム総数
		i, err := strconv.Atoi(sp[0])
		if err != nil {
			return 64, 0, fmt.Errorf("strconv error: sp0 (%s)", sp[0])
		}

		return i2, i, nil
	}

	// @ がない場合はデフォルトのスタックサイズ 64
	i, err := strconv.Atoi(s)
	if err != nil {
		return 64, 0, fmt.Errorf("strconv error: %s", s)
	}
	return 64, i, nil
}

// mod は Go の math.Mod を使って剰余を計算します。
func mod(x int, y int) int {
	return int(math.Mod(float64(x), float64(y)))
}

// calc はアイテム総数 amount を指定された size 単位で LC(54s), c(27s), st(1s), 個別 に換算します。
func calc(amount int, size int) string {
	lc := amount / (54 * size)
	amount = mod(amount, 54*size)
	sb := amount / (27 * size)
	amount = mod(amount, 27*size)
	st := amount / size
	amount = mod(amount, size)

	var res []string
	if lc > 0 {
		res = append(res, fmt.Sprintf("%dLC", lc))
	}
	if sb > 0 {
		res = append(res, fmt.Sprintf("%dc", sb))
	}
	if st > 0 {
		res = append(res, fmt.Sprintf("%dst", st))
	}
	if amount > 0 {
		res = append(res, fmt.Sprintf("%d", amount))
	}

	return strings.Join(res, "+")
}

// onMessage は Discord のメッセージイベントを処理します。
func onMessage(s *discordgo.Session, msg *discordgo.MessageCreate) {
	// ボット自身のメッセージは無視
	if msg.Author.Bot {
		return
	}

	// メッセージが "?=" で終わっているかチェック
	if strings.HasSuffix(msg.Content, "?=") {
		a := strings.TrimSuffix(msg.Content, "?=")
		size, amount, err := parseAmount(a)

		if err == nil {
			// 計算結果をリプライで送信
			s.ChannelMessageSendComplex(msg.ChannelID, &discordgo.MessageSend{
				AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
				Reference:       msg.Reference(),
				Content:         calc(amount, size),
			})
		} else {
			// パースエラーの場合、リアクションで通知
			s.MessageReactionAdd(msg.ChannelID, msg.ID, "❌")
		}
	}
}

func main() {
	// 環境変数 TOKEN からボットトークンを取得
	discord, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatal("Error creating Discord session: " + err.Error())
		return
	}

	// 必要な Intent の設定 (MESSAGE CONTENT INTENT は Developer Portal で有効化が必要)
	discord.Identify.Intents = discordgo.IntentMessageContent | discordgo.IntentGuildMessages
	discord.AddHandler(onMessage)

	// Discord への接続開始
	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening connection to Discord: " + err.Error())
	}

	// 🌟 RenderのためのヘルスチェックWebサーバーを起動 🌟
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // 環境変数PORTがない場合のデフォルト
	}

	// GoルーチンでWebサーバーを非同期に起動
	go func() {
		// ルートパス "/" にアクセスがあったら、200 OK とメッセージを返す
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK) // 200 OKを返す
			fmt.Fprintf(w, "Bot is healthy and connected to Discord.")
		})
		log.Printf("Starting web server for health checks on port: %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			// Webサーバーが落ちた場合は致命的なエラーとしてログに記録
			log.Fatalf("Web server failed: %v", err)
		}
	}()
	// 🌟 Webサーバー追加部分ここまで 🌟

	fmt.Println("Bot is now running.")

	// ボットの終了待機（Ctrl+C または SIGTERM）
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
	<-sigch

	// Discord 接続のクローズ
	err = discord.Close()
	if err != nil {
		log.Fatal("Error closing Discord connection: " + err.Error())
	}
}