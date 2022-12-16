package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

var hubspotAPIKey string
var azureURL string
var googleChatUrl string
var envAPIKey string

func main() {
	hubspotAPIKey = os.Getenv("HUBSPOT_API_KEY")
	azureURL = os.Getenv("AZURE_URL")
	googleChatUrl = os.Getenv("GOOGLE_CHAT_URL")
	envAPIKey = os.Getenv("API_KEY")
	r := gin.Default()
	r.POST("/generate-csv", GenerateCSV)
	r.Run()
}
