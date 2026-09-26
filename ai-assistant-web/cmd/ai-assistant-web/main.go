package main

import (
	assistantweb "aipanyi/ai-assistant-web"
	"log"
)

func main() {
	if err := assistantweb.Start(); err != nil {
		log.Fatalf("ai-assistant-web start failed: %v", err)
	}
}
