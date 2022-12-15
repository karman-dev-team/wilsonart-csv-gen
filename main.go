package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

type content struct {
	fname string
	ftype string
	fdata []byte
}

var HubspotAPIKey string

func main() {
	HubspotAPIKey = os.Getenv("HUBSPOT_API_KEY")
	r := gin.Default()
	r.POST("/generate-csv", GenerateCSV)
	r.Run()
}

func GenerateCSV(c *gin.Context) {
	var payloadBody HubspotPayload

	err := c.ShouldBindJSON(&payloadBody)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dealData, err := getDealData(payloadBody.ObjectID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyID, err := strconv.ParseInt(dealData.Associations.Companies.Results[0].ID, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyData, err := getCompanyData(companyID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lineItems, err := getAssociatedLineItems(payloadBody.ObjectID)
	if err != nil {
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
	// fmt.Println(fileText)
	fileText = fileText + "EOF"

	fileBinary := content{
		fname: fmt.Sprintf("%d-%s.txt", payloadBody.ObjectID, time.Now().Format("02-01-2006_1504")),
		ftype: "text",
		fdata: []byte(fileText),
	}

	var (
		buf = new(bytes.Buffer)
		w   = multipart.NewWriter(buf)
	)

	part, err := w.CreateFormFile(fileBinary.ftype, filepath.Base(fileBinary.fname))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	part.Write(fileBinary.fdata)
	w.Close()

	req, err := http.NewRequest("PUT", "https://testingcsv.blob.core.windows.net/?sv=2021-06-08&ss=bfqt&srt=sco&sp=w&se=2024-03-30T22:21:56Z&st=2022-12-15T14:21:56Z&spr=https&sig=y1CBxjHw4W%2B6HSRFeEbk8d7e%2BWQYyJZnXIJeOyk6WsQ%3D", buf)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Header.Add("Content-Type", w.FormDataContentType())
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println(res.Status)
	defer res.Body.Close()
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
