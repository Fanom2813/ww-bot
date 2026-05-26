// Command wapair is a standalone harness that de-risks the WhatsApp link
// before it is wired into the Wails app. It is now a thin consumer of the
// internal/wa package: it pairs via QR, persists the session, reconnects
// without re-pairing, and prints incoming messages and calls.
//
// Usage:
//
//	go run ./cmd/wapair      # first run: scan the QR; later runs: reconnects
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

	"wwbot/internal/wa"
)

const sessionDSN = "file:wwbot-session.db?_foreign_keys=on"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	client, err := wa.New(ctx, wa.Config{SessionDSN: sessionDSN, LogLevel: "INFO"})
	if err != nil {
		return err
	}

	// Consume normalized events. Start before connecting so no early event is missed.
	go consume(client.Events())

	if client.IsPaired() {
		fmt.Println("Already paired — reconnecting (no QR)...")
	} else {
		fmt.Println("Not paired yet — a QR code will appear below.")
	}
	if err := client.Start(ctx); err != nil {
		return err
	}

	fmt.Println("Listening. Press Ctrl+C to exit.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Println("\nDisconnecting...")
	client.Stop()
	return nil
}

func consume(events <-chan wa.Event) {
	for e := range events {
		switch ev := e.(type) {
		case wa.QR:
			fmt.Println("\nOn your phone: WhatsApp → Settings → Linked Devices → Link a Device, then scan:")
			qrterminal.GenerateHalfBlock(ev.Code, qrterminal.L, os.Stdout)
		case wa.Paired:
			fmt.Printf("\n🎉 Paired as %s — future runs won't need the QR.\n", ev.JID)
		case wa.Connected:
			fmt.Println("🔌 Connected to WhatsApp")
		case wa.LoggedOut:
			fmt.Println("🚪 Logged out — delete wwbot-session.db and re-pair.")
		case wa.Message:
			fmt.Printf("📩 [%s] %s (%s): %q\n", ev.Kind, ev.PushName, ev.SenderJID, ev.Text)
		case wa.Call:
			fmt.Printf("📞 Incoming call from %s\n", ev.FromJID)
		}
	}
}
