package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GenerateCSV(c *gin.Context) {
	apikey := c.GetHeader("x-api-key")
	if apikey != envAPIKey {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "x-api-key incorrect"})
		return
	}

	var payloadBody HubspotPayload

	err := c.ShouldBindJSON(&payloadBody)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(err.Error())
		}

		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dealData, err := getDealData(payloadBody.ObjectID)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error getting deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
		}
		createNoteHubspot(fmt.Sprintf("Error getting deal: %d", payloadBody.ObjectID), payloadBody.ObjectID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyID, err := strconv.ParseInt(dealData.Associations.Companies.Results[0].ID, 10, 64)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error no company associated to deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
		}
		createNoteHubspot(fmt.Sprintf("Error no company associated to deal: %d", payloadBody.ObjectID), payloadBody.ObjectID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyData, err := getCompanyData(companyID)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error getting company data of company: %d with error: %s", companyID, err.Error()))
		}
		createNoteHubspot(fmt.Sprintf("Error getting company data of company: %d", companyID), payloadBody.ObjectID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lineItems, err := getAssociatedLineItems(payloadBody.ObjectID)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error getting lineitems of deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
		}
		createNoteHubspot(fmt.Sprintf("Error getting lineitems of deal: %d", payloadBody.ObjectID), payloadBody.ObjectID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fileText := fmt.Sprintf(`"TYP","ORD","IN","2"
"HDR","1","2","2CH002","","","","%s","%s","%s","","","%s","%d","%s","%s","","00:00","00:00","","","%s","","","%d","1","1","","","","","","","","","",""
`, companyData.Properties.Name, companyData.Properties.Address, companyData.Properties.Address2, companyData.Properties.Zip, payloadBody.ObjectID, dealData.Properties.Createdate.Format("02/01/2006"), dealData.Properties.Createdate.Format("02/01/2006"), dealData.Properties.Createdate.Format("02/01/2006"), payloadBody.ObjectID)
	for i, item := range lineItems {
		fileText = fileText + fmt.Sprintf(`"LNE","%d","%s","","","","%s","%s","1","%s","","","","%d"
`, i, item.Properties.HsSku, item.Properties.Name, item.Properties.Name, item.Properties.Quantity, i)
	}
	fileText = fileText + "EOF"
	fileName := fmt.Sprintf("%d-%s.txt", payloadBody.ObjectID, time.Now().Format("02-01-2006_1504"))
	err = uploadTextFile(fileText, fileName)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error uploading text file with error: %s", err.Error()))
		}
		createNoteHubspot("Error uploading text file", payloadBody.ObjectID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = createNoteHubspot("CSV generation successful. <br> File upload successfully", payloadBody.ObjectID)
	if err != nil {
		if googleChatUrl != "" {
			logToGChat(fmt.Sprintf("Error uploading text file with error: %s", err.Error()))
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}
