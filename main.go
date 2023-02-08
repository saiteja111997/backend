package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/saiteja111997/videoChatApp/pkg/server"
)

func main() {
	port := "8080"
	r := gin.Default()
	r.Use(server.EnableCors)
	r.GET("/create", server.CreateRoomRequestHandler)
	r.GET("/join", server.JoinRoomRequestHandler)
	r.GET("/generateText", server.TranscribeAudioHandler)
	r.GET("/generateResult", server.GenerateFinalResult)
	// r.POST("/generateResultV2", server.GenerateFinalResultV2)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
