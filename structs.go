package main

import "time"

type HubspotPayload struct {
	PortalID     int    `json:"portalId"`
	ObjectType   string `json:"objectType"`
	ObjectTypeID string `json:"objectTypeId"`
	ObjectID     int    `json:"objectId"`
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
	HsTimestamp time.Time `json:"hs_timestamp"`
	HsNoteBody  string    `json:"hs_note_body"`
}

type hubspotNoteResponse struct {
	ID int `json:"id"`
}
