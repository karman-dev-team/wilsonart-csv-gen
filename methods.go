package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

func createNoteHubspot(errText string, dealId int) error {
	var noteBody hubspotNoteResponse

	body := struct {
		Properties hubspotNoteProperties
	}{
		Properties: hubspotNoteProperties{
			HsTimestamp: time.Now().UTC(),
			HsNoteBody:  errText,
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
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	noteResBody, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return errors.New("error creating hubspot note" + res.Status + "Error:" + string(noteResBody))
	}
	err = json.Unmarshal(noteResBody, &noteBody)
	if err != nil {
		return err
	}

	newReq, err := http.NewRequest("PUT", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/notes/%d/associations/deals/%d/214", noteBody.ID, dealId), bytes.NewBuffer(nil))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	newRes, err := client.Do(newReq)
	if err != nil {
		return err
	}
	assocResBody, _ := ioutil.ReadAll(newRes.Body)
	if res.StatusCode != http.StatusOK {
		return errors.New("error associating hubspot note" + res.Status + "Error:" + string(assocResBody))
	}

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

	s := strings.Split(azureURL, "?")

	conString := fmt.Sprintf("%s%s?%s", s[0], fName, s[1])

	req, err := http.NewRequest("PUT", conString, buf)
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
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(res.StatusCode)
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("error uploading file: %s", string(resBody))
	}
	defer res.Body.Close()
	return nil
}

func getDealData(dealID int) (HubspotDealBody, error) {
	client := &http.Client{}
	var dealBody HubspotDealBody
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/deals/%d?associations=companies", dealID), nil)
	if err != nil {
		return dealBody, errors.New(err.Error())
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return dealBody, errors.New(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return dealBody, errors.New("error getting deal" + resp.Status)
	}
	defer resp.Body.Close()

	dealRespBody, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(dealRespBody, &dealBody)
	if err != nil {
		return dealBody, err
	}
	return dealBody, nil
}

func getCompanyData(companyID int64) (HubspotCompanyBody, error) {
	client := &http.Client{}
	var companyBody HubspotCompanyBody
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/companies/%d?properties=name&properties=address&properties=address2&properties=zip", companyID), nil)
	if err != nil {
		return companyBody, errors.New(err.Error())
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return companyBody, errors.New(err.Error())
	}

	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return companyBody, errors.New(string(respBody))
	}
	json.Unmarshal(respBody, &companyBody)
	return companyBody, nil
}

func getAssociatedLineItems(dealID int) ([]HubspotLineItem, error) {
	client := &http.Client{}
	var associatedLineItemsBody HubspotAssociatedLineItems
	var lineItems []HubspotLineItem

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.hubapi.com/crm/v4/objects/deal/%d/associations/line_items", dealID), nil)
	if err != nil {
		return lineItems, errors.New(err.Error())
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
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
				wg.Done()
				return
			}
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", hubspotAPIKey))
			req.Header.Add("content-type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				lineItemErr <- err
				wg.Done()
				return
			}
			if resp.StatusCode != http.StatusOK {
				lineItemErr <- errors.New("error getting associated line items")
				wg.Done()
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
