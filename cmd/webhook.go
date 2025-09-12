package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type IncomingCall struct {
	DialogID  string `json:"dialog_id"`
	CreatedAt string `json:"created_at"`
	Caller    string `json:"caller"`
	Callee    string `json:"callee"`
	Event     string `json:"event"`
	Offer     string `json:"offer"`
}

func serveWebhook(parent context.Context, option CreateClientOption, addr, prefix string) {
	server := gin.Default()
	server.POST(prefix, func(c *gin.Context) {
		var form IncomingCall
		if err := c.ShouldBindJSON(&form); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		option.Logger.WithFields(logrus.Fields{
			"dialogId":  form.DialogID,
			"createdAt": form.CreatedAt,
			"caller":    form.Caller,
			"callee":    form.Callee,
			"event":     form.Event,
		}).Info("Incoming call")

		client := createClient(parent, option, form.DialogID)

		go func() {
			ctx, cancel := context.WithCancel(parent)
			defer cancel()
			err := client.Connect("sip")
			if err != nil {
				option.Logger.Errorf("Failed to connect to server: %v", err)
			}
			defer client.Shutdown()
			client.Accept(option.CallOption)
			time.Sleep(300 * time.Millisecond)
			client.TTS(option.Greeting, "", "", true, false, nil, nil)
			<-ctx.Done()
		}()

		c.JSON(200, gin.H{"message": "OK"})
	})
	server.Run(addr)
}
