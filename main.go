package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type HubspotPayload struct {
	PortalID     int    `json:"portalId"`
	ObjectType   string `json:"objectType"`
	ObjectTypeID string `json:"objectTypeId"`
	ObjectID     int64  `json:"objectId"`
}

type HubspotDealBody struct {
	Associations struct {
		Companies struct {
			Results []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"results"`
		} `json:"companies"`
	} `json:"associations"`
	Properties struct {
		Createdate time.Time `json:"createdate"`
		Dealname   string    `json:"dealname"`
	} `json:"properties"`
}

type HubspotCompanyBody struct {
	Properties struct {
		Address  string `json:"address"`
		Address2 string `json:"address2"`
		Name     string `json:"name"`
		Zip      string `json:"zip"`
	} `json:"properties"`
}

type HubspotAssociatedLineItems struct {
	Results []AssociationResult `json:"results"`
}

type AssociationResult struct {
	ToObjectID int64 `json:"toObjectId"`
}

type HubspotLineItem struct {
	Properties struct {
		HsSku    string `json:"hs_sku"`
		Name     string `json:"name"`
		Quantity string `json:"quantity"`
	} `json:"properties"`
}

type hubspotNoteProperties struct {
	hsTimestamp time.Time
	hsNoteBody  string
}

var HubspotAPIKey string
var azureSignature string
var azureContainer string
var azureBucket string
var googleChatUrl string

func main() {
	HubspotAPIKey = os.Getenv("HUBSPOT_API_KEY")
	azureSignature = os.Getenv("AZURE_SIG")
	azureBucket = os.Getenv("AZURE_BUCKET")
	azureContainer = os.Getenv("AZURE_CONTAINER")
	googleChatUrl = os.Getenv("GOOGLE_CHAT_URL")
	r := gin.Default()
	r.POST("/generate-csv", GenerateCSV)
	r.Run()
}

func GenerateCSV(c *gin.Context) {
	var payloadBody HubspotPayload

	err := c.ShouldBindJSON(&payloadBody)
	if err != nil {
		logToGChat(err.Error())
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dealData, err := getDealData(payloadBody.ObjectID)
	if err != nil {
		logToGChat(fmt.Sprintf("Error getting deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyID, err := strconv.ParseInt(dealData.Associations.Companies.Results[0].ID, 10, 64)
	if err != nil {
		logToGChat(fmt.Sprintf("Error no company associated to deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyData, err := getCompanyData(companyID)
	if err != nil {
		logToGChat(fmt.Sprintf("Error getting company data of company: %d with error: %s", companyID, err.Error()))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lineItems, err := getAssociatedLineItems(payloadBody.ObjectID)
	if err != nil {
		logToGChat(fmt.Sprintf("Error getting lineitems of deal: %d with error: %s", payloadBody.ObjectID, err.Error()))
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
		logToGChat(fmt.Sprintf("Error uploading text file with error: %s", err.Error()))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func createNoteHubspot(errText string, dealId int64) error {
	body := struct {
		Properties hubspotNoteProperties
	}{
		Properties: hubspotNoteProperties{
			hsTimestamp: time.Now().UTC(),
			hsNoteBody:  errText,
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.hubapi.com/crm/v3/objects/notes", bytes.NewBuffer(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", ""))
	req.Header.Add("content-type", "application/json")
	return nil
}

func logToGChat(errText string) {
	body := struct {
		Text string `json:"text"`
	}{
		Text: "WilsonArt Error:" + errText,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", googleChatUrl, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
}

func uploadTextFile(fData string, fName string) error {

	buf := new(bytes.Buffer)

	buf.WriteString(fData)

	req, err := http.NewRequest("PUT", fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?sv=2021-06-08&ss=bfqt&srt=sco&sp=w&se=2024-03-30T22:21:56Z&st=2022-12-15T14:21:56Z&sip=213.122.115.161&spr=https&sig=%s", azureBucket, azureContainer, fName, azureSignature), buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("x-ms-blob-type", "BlockBlob")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func getDealData(dealID int64) (HubspotDealBody, error) {
	client := &http.Client{}
	var dealBody HubspotDealBody
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/deals/%d?associations=companies", dealID), nil)
	if err != nil {
		return dealBody, errors.New(err.Error())
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", HubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return dealBody, errors.New(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return dealBody, errors.New("error getting company")
	}
	defer resp.Body.Close()

	dealRespBody, _ := ioutil.ReadAll(resp.Body)

	json.Unmarshal(dealRespBody, &dealBody)
	return dealBody, nil
}

func getCompanyData(companyID int64) (HubspotCompanyBody, error) {
	client := &http.Client{}
	var companyBody HubspotCompanyBody
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/companies/%d?properties=name&properties=address&properties=address2&properties=zip", companyID), nil)
	if err != nil {
		return companyBody, errors.New(err.Error())
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", HubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return companyBody, errors.New(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return companyBody, errors.New("error getting deal")
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(respBody, &companyBody)
	return companyBody, nil
}

func getAssociatedLineItems(dealID int64) ([]HubspotLineItem, error) {
	client := &http.Client{}
	var associatedLineItemsBody HubspotAssociatedLineItems
	var lineItems []HubspotLineItem

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v4/objects/deal/%d/associations/line_items", dealID), nil)
	if err != nil {
		return lineItems, errors.New(err.Error())
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", HubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return lineItems, errors.New(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return lineItems, errors.New("error getting associated line items")
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(respBody, &associatedLineItemsBody)

	var wg sync.WaitGroup
	wg.Add(len(associatedLineItemsBody.Results))
	var lineItemErr = make(chan error, 1)
	for _, item := range associatedLineItemsBody.Results {
		go func(i AssociationResult) {
			var lineItem HubspotLineItem
			req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/line_items/%d?properties=name&properties=hs_product_id&properties=quantity&properties=hs_sku", i.ToObjectID), nil)
			if err != nil {
				lineItemErr <- err
				return
			}
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", HubspotAPIKey))
			req.Header.Add("content-type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				lineItemErr <- err
				return
			}
			if resp.StatusCode != http.StatusOK {
				lineItemErr <- errors.New("error getting associated line items")
				return
			}
			defer resp.Body.Close()

			respBody, _ := ioutil.ReadAll(resp.Body)
			json.Unmarshal(respBody, &lineItem)
			lineItems = append(lineItems, lineItem)

			wg.Done()

		}(item)

	}
	wg.Wait()

	select {
	case err := <-lineItemErr:
		return []HubspotLineItem{}, err
	default:
		return lineItems, nil
	}
}
