// Command wapair is a standalone harness that de-risks the WhatsApp link
// before it is wired into the Wails app. It pairs a device via QR code,
// persists the session in SQLite, and on subsequent runs reconnects using the
// stored session WITHOUT re-pairing. It also logs incoming messages, voice
// notes and call offers so we can confirm those events arrive.
//
// Usage:
//
//	go run ./cmd/wapair            # first run: scan the QR with your phone
//	go run ./cmd/wapair            # later runs: reconnects, no QR
//
// The session lives in ./wwbot-session.db (gitignored). Delete it to unpair.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	_ "github.com/mattn/go-sqlite3" // registers the "sqlite3" database/sql driver

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// sessionDSN is the SQLite store for the encrypted WhatsApp session/keys.
const sessionDSN = "file:wwbot-session.db?_foreign_keys=on"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Open the session store. The dialect string ("sqlite3") is also the
	// database/sql driver name, provided by the blank-imported mattn driver.
	container, err := sqlstore.New(ctx, "sqlite3", sessionDSN, waLog.Stdout("DB", "WARN", true))
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}

	// GetFirstDevice returns the saved device, or a fresh one if none exists.
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("load device: %w", err)
	}

	client := whatsmeow.NewClient(device, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// No stored session yet — pair via QR.
		if err := pair(ctx, client); err != nil {
			return err
		}
	} else {
		// Already paired — reconnect with the stored session, no QR needed.
		fmt.Printf("Already paired as %s — reconnecting (no QR)...\n", client.Store.ID)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("connect: %w", err)
		}
	}

	fmt.Println("\n✅ Connected. Listening for messages and calls. Press Ctrl+C to exit.")

	// Block until interrupted, then disconnect cleanly.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Println("\nDisconnecting...")
	client.Disconnect()
	return nil
}

// pair runs the QR-login flow: the channel must be obtained before Connect.
func pair(ctx context.Context, client *whatsmeow.Client) error {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("get QR channel: %w", err)
	}
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			fmt.Println("\nOn your phone: WhatsApp → Settings → Linked Devices → Link a Device,")
			fmt.Println("then scan this QR code:")
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "success":
			fmt.Println("\n🎉 Paired! Session saved — future runs won't need the QR.")
		case "timeout":
			return fmt.Errorf("QR scan timed out; re-run to try again")
		case "error":
			return fmt.Errorf("pairing error: %w", evt.Error)
		default:
			fmt.Printf("login event: %s\n", evt.Event)
		}
	}
	return nil
}

// eventHandler logs the event types relevant to the bot, so we can confirm
// real messages, voice notes and call offers actually arrive.
func eventHandler(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		text := v.Message.GetConversation()
		if ext := v.Message.GetExtendedTextMessage(); ext != nil {
			text = ext.GetText()
		}
		kind := "text"
		switch {
		case v.Message.GetAudioMessage() != nil:
			kind = "voice"
		case v.Message.GetImageMessage() != nil:
			kind = "image"
		}
		from := v.Info.PushName
		if from == "" {
			from = v.Info.Sender.User
		}
		fmt.Printf("📩 [%s] %s (%s): %q\n", kind, from, v.Info.Sender.User, text)

	case *events.CallOffer:
		fmt.Printf("📞 Incoming call from %s\n", v.From.User)

	case *events.CallOfferNotice:
		fmt.Printf("📞 Incoming %s call from %s\n", v.Media, v.From.User)

	case *events.Connected:
		fmt.Println("🔌 Connected to WhatsApp servers")

	case *events.LoggedOut:
		fmt.Println("🚪 Logged out — session invalidated. Delete wwbot-session.db and re-pair.")
	}
}
