package model3

import (
	"encoding/json"
	"time"
)

type TestAttachmentDetails struct {
	TestIdentifier string       `json:"testIdentifier"`
	Attachments    []Attachment `json:"attachments"`
}

type Timestamp time.Time

type Attachment struct {
	ExportedFileName           string    `json:"exportedFileName"`
	SuggestedHumanReadableName string    `json:"suggestedHumanReadableName"`
	IsAssociatedWithFailure    bool      `json:"isAssociatedWithFailure"`
	Timestamp                  Timestamp `json:"timestamp"`
	ConfigurationName          string    `json:"configurationName"`
	DeviceName                 string    `json:"deviceName"`
	DeviceID                   string    `json:"deviceId"`
	RepetitionNumber           int       `json:"repetitionNumber"`
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp float64
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}

	*t = Timestamp(time.Unix(int64(timestamp), 0))

	return nil
}
