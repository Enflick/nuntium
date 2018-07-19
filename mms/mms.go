/*
 * Copyright 2014 Canonical Ltd.
 *
 * Authors:
 * Sergio Schvezov: sergio.schvezov@cannical.com
 *
 * This file is part of mms.
 *
 * mms is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; version 3.
 *
 * mms is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 * Github https://github.com/ubuntu-phonedations/nuntium/tree/master/mms
 */

package mms

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// MMS Field names HeaderFrom OMA-WAP-MMS section 7.3 Table 12
const (
	HeaderBCC                       = 0x01
	HeaderCC                        = 0x02
	HeaderXMmsContentLocation       = 0x03
	HeaderContentType               = 0x04
	HeaderDate                      = 0x05
	HeaderXMmsDeliveryReport        = 0x06
	HeaderXMmsDeliveryTime          = 0x07
	HeaderXMmsExpiry                = 0x08
	HeaderFrom                      = 0x09
	HeaderXMmsMessageClass          = 0x0A
	HeaderMessageID                 = 0x0B
	HeaderXMmsMessageType           = 0x0C
	HeaderXMmsMmsVersion            = 0x0D
	HeaderXMmsMessageSize           = 0x0E
	HeaderxMmsPriority              = 0x0F
	HeaderXMmsReadReport            = 0x10
	HeaderXMmsReportAllowed         = 0x11
	HeaderXMmsResponseStatus        = 0x12
	HeaderXMmsResponseText          = 0x13
	HeaderXMmsSenderVisibility      = 0x14
	HeaderXMmsStatus                = 0x15
	HeaderSubject                   = 0x16
	HeaderTo                        = 0x17
	HeaderXMmsTransactionID         = 0x18
	HeaderXMmsRetrieveStatus        = 0x19
	HeaderXMmsRetrieveText          = 0x1A
	HeaderXMmsReadStatus            = 0x1B
	HeaderXMmsReplyCharging         = 0x1C
	HeaderXMmsReplyChargingDeadline = 0x1D
	HeaderXMmsReplyChargingID       = 0x1E
	HeaderXMmsReplyChargingSize     = 0x1F
	HeaderXMmsPreviouslySentBy      = 0x20
	HeaderXMmsPreviouslySentDate    = 0x21
)

// MMS Content Type Assignments OMA-WAP-MMS section 7.3 Table 13
const (
	pushApplicationID = 4
	vndWapMmsMessage  = "application/vnd.wap.mms-message"
)

const (
	typeSendReq         = 0x80
	typeSendConf        = 0x81
	typeNotificationInd = 0x82
	typeNotifyrespInd   = 0x83
	typeRetrieveConf    = 0x84
	typeAcknowledgeInd  = 0x85
	typeDeliveryInd     = 0x86
)

const (
	MMSMessageVersion1_0 = 0x90
	MMSMessageVersion1_1 = 0x91
	MMSMessageVersion1_2 = 0x92
	MMSMessageVersion1_3 = 0x93
)

// Delivery Report defined in OMA-WAP-MMS section 7.2.6
const (
	DeliveryReportYes byte = 128
	DeliveryReportNo  byte = 129
)

// Expiry tokens defined in OMA-WAP-MMS section 7.2.10
const (
	ExpiryTokenAbsolute byte = 128
	ExpiryTokenRelative byte = 129
)

// From tokens defined in OMA-WAP-MMS section 7.2.11
const (
	tokenAddressPresent = 0x80
	tokenInsertAddress  = 0x81
)

// Message classes defined in OMA-WAP-MMS section 7.2.14
const (
	ClassPersonal      byte = 128
	ClassAdvertisement byte = 129
	ClassInformational byte = 130
	ClassAuto          byte = 131
)

// Report Report defined in OMA-WAP-MMS 7.2.20
const (
	ReadReportYes byte = 128
	ReadReportNo  byte = 129
)

// Report Allowed defined in OMA-WAP-MMS section 7.2.26
const (
	ReportAllowedYes byte = 128
	ReportAllowedNo  byte = 129
)

// Response Status defined in OMA-WAP-MMS section 7.2.27
//
// An MMS Client MUST react the same HeaderTo a value in range 196 HeaderTo 223 as it
// does HeaderTo the value 192 (Error-transient-failure).
//
// An MMS Client MUST react the same HeaderTo a value in range 234 HeaderTo 255 as it
// does HeaderTo the value 224 (Error-permanent-failure).
//
// Any other values SHALL NOT be used. They are reserved for future use.
// An MMS Client that receives such a reserved value MUST react the same
// as it does HeaderTo the value 224 (Error-permanent-failure).
const (
	ResponseStatusOk                            byte = 128
	ResponseStatusErrorUnspecified              byte = 129 // Obsolete
	ResponseStatusErrorServiceDenied            byte = 130 // Obsolete
	ResponseStatusErrorMessageFormatCorrupt     byte = 131 // Obsolete
	ResponseStatusErrorSendingAddressUnresolved byte = 132 // Obsolete
	ResponseStatusErrorMessageNotFound          byte = 133 // Obsolete
	ResponseStatusErrorNetworkProblem           byte = 134 // Obsolete
	ResponseStatusErrorContentNotAccepted       byte = 135 // Obsolete
	ResponseStatusErrorUnsupportedMessage       byte = 136

	ResponseStatusErrorTransientFailure           byte = 192
	ResponseStatusErrorTransientAddressUnresolved byte = 193
	ResponseStatusErrorTransientMessageNotFound   byte = 194
	ResponseStatusErrorTransientNetworkProblem    byte = 195

	ResponseStatusErrorTransientMaxReserved byte = 223

	ResponseStatusErrorPermanentFailure                         byte = 224
	ResponseStatusErrorPermanentServiceDenied                   byte = 225
	ResponseStatusErrorPermanentMessageFormatCorrupt            byte = 226
	ResponseStatusErrorPermanentAddressUnresolved               byte = 227
	ResponseStatusErrorPermanentMessageNotFound                 byte = 228
	ResponseStatusErrorPermanentContentNotAccepted              byte = 229
	ResponseStatusErrorPermanentReplyChargingLimitationsNotMet  byte = 230
	ResponseStatusErrorPermanentReplyChargingRequestNotAccepted byte = 231
	ResponseStatusErrorPermanentReplyChargingForwardingDenied   byte = 232
	ResponseStatusErrorPermanentReplyChargingNotSupported       byte = 233

	ResponseStatusErrorPermamentMaxReserved byte = 255
)

// Status defined in OMA-WAP-MMS section 7.2.23
const (
	statusExpired      = 128
	statusRetrieved    = 129
	statusRejected     = 130
	statusDeferred     = 131
	statusUnrecognized = 132
)

// MSendReq holds a m-send.req message defined in
// OMA-WAP-MMS-ENC-v1.1 section 6.1.1
type MSendReq struct {
	UUID             string `encode:"no"`
	Type             byte
	TransactionID    string
	Version          byte
	Date             uint64 `encode:"optional"`
	From             string
	To               []string
	Cc               string `encode:"no"`
	Bcc              string `encode:"no"`
	Subject          string `encode:"optional"`
	Class            byte   `encode:"optional"`
	Expiry           uint64 `encode:"optional"`
	DeliveryTime     uint64 `encode:"optional"`
	Priority         byte   `encode:"optional"`
	SenderVisibility byte   `encode:"optional"`
	DeliveryReport   byte   `encode:"optional"`
	ReadReport       byte   `encode:"optional"`
	ContentTypeStart string `encode:"no"`
	ContentTypeType  string `encode:"no"`
	ContentType      string
	Attachments      []*Attachment `encode:"no"`
}

// MSendConf holds a m-send.conf message defined in
// OMA-WAP-MMS-ENC section 6.1.2
type MSendConf struct {
	Type           byte
	TransactionID  string
	Version        byte
	ResponseStatus byte
	ResponseText   string
	MessageID      string
}

// MNotificationInd holds a m-notification.ind message defined in
// OMA-WAP-MMS-ENC section 6.2
type MNotificationInd struct {
	Reader
	UUID                                 string
	Type, Version, Class, DeliveryReport byte
	ReplyCharging, ReplyChargingDeadline byte
	Priority                             byte
	ReplyChargingID                      string
	TransactionID, ContentLocation       string
	From, Subject                        string
	Expiry, Size                         uint64
}

// MNotifyRespInd holds a m-notifyresp.ind message defined in
// OMA-WAP-MMS-ENC-v1.1 section 6.2
type MNotifyRespInd struct {
	UUID          string `encode:"no"`
	Type          byte
	TransactionID string
	Version       byte
	Status        byte
	ReportAllowed byte `encode:"optional"`
}

// MRetrieveConf holds a m-retrieve.conf message defined in
// OMA-WAP-MMS-ENC-v1.1 section 6.3
type MRetrieveConf struct {
	Reader
	UUID                                       string
	Type, Version, Status, Class, Priority     byte
	ReplyCharging, ReplyChargingDeadline       byte
	ReplyChargingID                            string
	ReadReport, RetrieveStatus, DeliveryReport byte
	TransactionID, MessageID, RetrieveText     string
	From, Cc, Subject                          string
	To                                         []string
	ReportAllowed                              byte
	Date                                       uint64
	Content                                    Attachment
	Attachments                                []Attachment
	Data                                       []byte
}

// Reader interface
type Reader interface{}

// Writer interface
type Writer interface{}

// NewMSendReq creates a personal message with a normal priority and no read report
func NewMSendReq(recipients []string, attachments []*Attachment, deliveryReport bool) *MSendReq {
	for i := range recipients {
		recipients[i] += "/TYPE=PLMN"
	}
	uuid := genUUID()

	orderedAttachments, smilStart, smilType := processAttachments(attachments)

	return &MSendReq{
		Type:          typeSendReq,
		To:            recipients,
		TransactionID: uuid,
		Version:       MMSMessageVersion1_1,
		UUID:          uuid,
		Date:          getDate(),
		// this will expire the message in 7 days
		Expiry:           uint64(time.Duration(time.Hour * 24 * 7).Seconds()),
		DeliveryReport:   getDeliveryReport(deliveryReport),
		ReadReport:       ReadReportNo,
		Class:            ClassPersonal,
		ContentType:      "application/vnd.wap.multipart.mixed",
		ContentTypeStart: smilStart,
		ContentTypeType:  smilType,
		Attachments:      orderedAttachments,
	}
}

// NewMSendConf create a new instance MSendConf
func NewMSendConf() *MSendConf {
	return &MSendConf{
		Type: typeSendConf,
	}
}

// NewMNotificationInd create a new instance of MNotification Ind
func NewMNotificationInd() *MNotificationInd {
	return &MNotificationInd{Type: typeNotificationInd, UUID: genUUID()}
}

// IsLocal func
func (mNotificationInd *MNotificationInd) IsLocal() bool {
	return strings.HasPrefix(mNotificationInd.ContentLocation, "http://localhost:9191/mms")
}

// NewMNotifyRespInd create a new instance of MNotifyRespInd
func (mNotificationInd *MNotificationInd) NewMNotifyRespInd(status byte, deliveryReport bool) *MNotifyRespInd {
	return &MNotifyRespInd{
		Type:          typeNotifyrespInd,
		UUID:          mNotificationInd.UUID,
		TransactionID: mNotificationInd.TransactionID,
		Version:       mNotificationInd.Version,
		Status:        status,
		ReportAllowed: getReportAllowed(deliveryReport),
	}
}

// NewMNotifyRespInd create a new instance of MNotifyRespInd
func (mRetrieveConf *MRetrieveConf) NewMNotifyRespInd(deliveryReport bool) *MNotifyRespInd {
	return &MNotifyRespInd{
		Type:          typeNotifyrespInd,
		UUID:          mRetrieveConf.UUID,
		TransactionID: mRetrieveConf.TransactionID,
		Version:       mRetrieveConf.Version,
		Status:        statusRetrieved,
		ReportAllowed: getReportAllowed(deliveryReport),
	}
}

// NewMNotifyRespInd create a new instance of MNotifyRespInd
func NewMNotifyRespInd() *MNotifyRespInd {
	return &MNotifyRespInd{Type: typeNotifyrespInd}
}

// NewMRetrieveConf create a new instance of MRetrieveConf
func NewMRetrieveConf(uuid string) *MRetrieveConf {
	return &MRetrieveConf{Type: typeRetrieveConf, UUID: uuid}
}

// genUUID func
func genUUID() string {
	var id string
	random, err := os.Open("/dev/urandom")
	if err != nil {
		id = "1234567890ABCDEF"
	} else {
		defer random.Close()
		b := make([]byte, 16)
		random.Read(b)
		id = fmt.Sprintf("%x", b)
	}
	return id
}

// ErrTransient error
var ErrTransient = errors.New("Error-transient-failure")

// ErrPermanent error
var ErrPermanent = errors.New("Error-permament-failure")

// Status func
func (mSendConf *MSendConf) Status() error {
	s := mSendConf.ResponseStatus
	// these are case by case Response Status and we need HeaderTo determine each one
	switch s {
	case ResponseStatusOk:
		return nil
	case ResponseStatusErrorUnspecified:
		return ErrTransient
	case ResponseStatusErrorServiceDenied:
		return ErrTransient
	case ResponseStatusErrorMessageFormatCorrupt:
		return ErrPermanent
	case ResponseStatusErrorSendingAddressUnresolved:
		return ErrPermanent
	case ResponseStatusErrorMessageNotFound:
		// this could be ErrTransient or ErrPermanent
		return ErrPermanent
	case ResponseStatusErrorNetworkProblem:
		return ErrTransient
	case ResponseStatusErrorContentNotAccepted:
		return ErrPermanent
	case ResponseStatusErrorUnsupportedMessage:
		return ErrPermanent
	}

	// these are the Response Status we can group
	if s >= ResponseStatusErrorTransientFailure && s <= ResponseStatusErrorTransientMaxReserved {
		return ErrTransient
	} else if s >= ResponseStatusErrorPermanentFailure && s <= ResponseStatusErrorPermamentMaxReserved {
		return ErrPermanent
	}

	// any case not handled is a permanent error
	return ErrPermanent
}

// getReadReport func
func getReadReport(v bool) (read byte) {
	if v {
		read = ReadReportYes
	} else {
		read = ReadReportNo
	}
	return read
}

// getDeliveryReport func
func getDeliveryReport(v bool) (delivery byte) {
	if v {
		delivery = DeliveryReportYes
	} else {
		delivery = DeliveryReportNo
	}
	return delivery
}

// getReportAllowed func
func getReportAllowed(v bool) (allowed byte) {
	if v {
		allowed = ReportAllowedYes
	} else {
		allowed = ReportAllowedNo
	}
	return allowed
}

// getDate func
func getDate() (date uint64) {
	d := time.Now().Unix()
	if d > 0 {
		date = uint64(d)
	}
	return date
}

// processAttachments func
func processAttachments(a []*Attachment) (oa []*Attachment, smilStart, smilType string) {
	oa = make([]*Attachment, 0, len(a))
	for i := range a {
		if strings.HasPrefix(a[i].MediaType, "application/smil") {
			oa = append([]*Attachment{a[i]}, oa...)
			var err error
			smilStart, err = getSmilStart(a[i].Data)
			if err != nil {
				log.Println("Cannot set content type start:", err)
			}
			smilType = "application/smil"
		} else {
			oa = append(oa, a[i])
		}
	}
	return oa, smilStart, smilType
}
