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
	"fmt"
	"log"
	"reflect"
)

// NewDecoder will create a new instance of Decoder
func NewDecoder(data []byte) *Decoder {
	return &Decoder{Data: data}
}

// Decoder struct
type Decoder struct {
	Data   []byte
	Offset int
	log    string
}

// setPduField will set the pdu field
func (dec *Decoder) setPduField(pdu *reflect.Value, name string, v interface{},
	setter func(*reflect.Value, interface{})) {

	if name != "" {
		field := pdu.FieldByName(name)
		if field.IsValid() {
			setter(&field, v)
			dec.log = dec.log + fmt.Sprintf("Setting %s HeaderTo %s\n", name, v)
		} else {
			log.Println("Field", name, "not in decoding structure")
		}
	}
}

func setterString(field *reflect.Value, v interface{}) { field.SetString(v.(string)) }
func setterUint64(field *reflect.Value, v interface{}) { field.SetUint(v.(uint64)) }
func setterSlice(field *reflect.Value, v interface{})  { field.SetBytes(v.([]byte)) }

// ReadEncodedString read encoded string
func (dec *Decoder) ReadEncodedString(reflectedPdu *reflect.Value, hdr string) (string, error) {
	var length uint64
	var err error
	switch {
	case dec.Data[dec.Offset+1] < shortLengthMax:
		var l byte
		l, err = dec.ReadShortInteger(nil, "")
		length = uint64(l)
	case dec.Data[dec.Offset+1] == lengthQuote:
		dec.Offset++
		length, err = dec.ReadUintVar(nil, "")
	}
	if err != nil {
		return "", err
	}
	if length != 0 {
		charset, err := dec.ReadCharset(nil, "")
		if err != nil {
			return "", err
		}
		dec.log = dec.log + fmt.Sprintf("Next string encoded with: %s\n", charset)
	}
	var str string
	if str, err = dec.ReadString(reflectedPdu, hdr); err != nil {
		return "", err
	}
	return str, nil
}

//ReadQ func
func (dec *Decoder) ReadQ(reflectedPdu *reflect.Value) error {
	v, err := dec.ReadUintVar(nil, "")
	if err != nil {
		return err
	}
	q := float64(v)
	if q > 100 {
		q = (q - 100) / 1000
	} else {
		q = (q - 1) / 100
	}
	reflectedPdu.FieldByName("Q").SetFloat(q)
	return nil
}

// ReadLength reads the length HeaderFrom the next position according HeaderTo section
// 8.4.2.2 of WAP-230-WSP-20010705-a.
//
// Value-length = Short-length | (Length-quote Length)
// ; Value length is used HeaderTo indicate the length of the value HeaderTo follow
// Short-length = <Any octet 0-30> (0x7f HeaderTo check for short)
// Length-quote = <Octet 31>
// Length = Uintvar-integer
func (dec *Decoder) ReadLength(reflectedPdu *reflect.Value) (length uint64, err error) {
	switch {
	case dec.Data[dec.Offset+1]&0x7f <= shortLengthMax:
		l, err := dec.ReadShortInteger(nil, "")
		v := uint64(l)
		if reflectedPdu != nil {
			reflectedPdu.FieldByName("Length").SetUint(v)
		}
		return v, err
	case dec.Data[dec.Offset+1] == lengthQuote:
		dec.Offset++
		var hdr string
		if reflectedPdu != nil {
			hdr = "Length"
		}
		return dec.ReadUintVar(reflectedPdu, hdr)
	}
	return 0, fmt.Errorf("Unhandled length %#x @%d", dec.Data[dec.Offset+1], dec.Offset)
}

// ReadCharset read charset
func (dec *Decoder) ReadCharset(reflectedPdu *reflect.Value, hdr string) (string, error) {
	var charset string

	if dec.Data[dec.Offset] == anyCharset {
		dec.Offset++
		charset = "*"
	} else {
		charCode, err := dec.ReadInteger(nil, "")
		if err != nil {
			return "", err
		}
		var ok bool
		if charset, ok = CHARSETS[charCode]; !ok {
			return "", fmt.Errorf("Cannot find matching charset for %#x == %d", charCode, charCode)
		}
	}
	if hdr != "" {
		reflectedPdu.FieldByName("Charset").SetString(charset)
	}
	return charset, nil
}

// ReadMediaType read media type
func (dec *Decoder) ReadMediaType(reflectedPdu *reflect.Value, hdr string) (err error) {
	var mediaType string
	var endOffset int
	origOffset := dec.Offset

	if dec.Data[dec.Offset+1] <= shortLengthMax || dec.Data[dec.Offset+1] == lengthQuote {
		length, err := dec.ReadLength(nil)
		if err != nil {
			return err
		}
		endOffset = int(length) + dec.Offset
	}

	if dec.Data[dec.Offset+1] >= textMin && dec.Data[dec.Offset+1] <= textMax {
		if mediaType, err = dec.ReadString(nil, ""); err != nil {
			return err
		}
	} else if mt, err := dec.ReadInteger(nil, ""); err == nil && len(contentTypes) > int(mt) {
		mediaType = contentTypes[mt]
	} else {
		return fmt.Errorf("cannot decode media type for field beginning with %#x@%d", dec.Data[origOffset], origOffset)
	}

	// skip the rest of the content type params
	if endOffset > 0 {
		dec.Offset = endOffset
	}

	reflectedPdu.FieldByName(hdr).SetString(mediaType)
	dec.log = dec.log + fmt.Sprintf("%s: %s\n", hdr, mediaType)

	return nil
}

// ReadTo func
func (dec *Decoder) ReadTo(reflectedPdu *reflect.Value) error {
	// field in the MMS protocol
	toField, err := dec.ReadEncodedString(reflectedPdu, "")
	if err != nil {
		return err
	}
	// field in the golang structure
	to := reflectedPdu.FieldByName("To")
	toSlice := reflect.Append(to, reflect.ValueOf(toField))
	reflectedPdu.FieldByName("To").Set(toSlice)
	return err
}

// ReadString func
func (dec *Decoder) ReadString(reflectedPdu *reflect.Value, hdr string) (string, error) {
	dec.Offset++
	if dec.Data[dec.Offset] == 34 { // Skip the quote char(34) == "
		dec.Offset++
	}
	begin := dec.Offset
	for ; len(dec.Data) > dec.Offset; dec.Offset++ {
		if dec.Data[dec.Offset] == 0 {
			break
		}
	}
	if len(dec.Data) == dec.Offset {
		return "", fmt.Errorf("reached end of data while trying HeaderTo read string: %s", dec.Data[begin:])
	}
	v := string(dec.Data[begin:dec.Offset])
	dec.setPduField(reflectedPdu, hdr, v, setterString)

	return v, nil
}

// ReadShortInteger func
func (dec *Decoder) ReadShortInteger(reflectedPdu *reflect.Value, hdr string) (byte, error) {
	dec.Offset++
	/*
		TODO fix use of short when not short
		if dec.Data[dec.Offset] & 0x80 == 0 {
			return 0, fmt.Errorf("Data on offset %d with value %#x is not a short integer", dec.Offset, dec.Data[dec.Offset])
		}
	*/
	v := dec.Data[dec.Offset] & 0x7F
	dec.setPduField(reflectedPdu, hdr, uint64(v), setterUint64)

	return v, nil
}

// ReadByte func
func (dec *Decoder) ReadByte(reflectedPdu *reflect.Value, hdr string) (byte, error) {
	dec.Offset++
	v := dec.Data[dec.Offset]
	dec.setPduField(reflectedPdu, hdr, uint64(v), setterUint64)

	return v, nil
}

// ReadBoundedBytes func
func (dec *Decoder) ReadBoundedBytes(reflectedPdu *reflect.Value, hdr string, end int) ([]byte, error) {
	v := []byte(dec.Data[dec.Offset:end])
	dec.setPduField(reflectedPdu, hdr, v, setterSlice)
	dec.Offset = end - 1

	return v, nil
}

// ReadUintVar is a variable lenght uint of up HeaderTo 5 octects long where
// more octects available are indicated with the most significant bit
// set HeaderTo 1
func (dec *Decoder) ReadUintVar(reflectedPdu *reflect.Value, hdr string) (value uint64, err error) {
	dec.Offset++
	for dec.Data[dec.Offset]>>7 == 0x01 {
		value = value << 7
		value |= uint64(dec.Data[dec.Offset] & 0x7F)
		dec.Offset++
	}

	value = value << 7
	value |= uint64(dec.Data[dec.Offset] & 0x7F)
	dec.setPduField(reflectedPdu, hdr, value, setterUint64)

	return value, nil
}

// ReadInteger func
func (dec *Decoder) ReadInteger(reflectedPdu *reflect.Value, hdr string) (uint64, error) {
	param := dec.Data[dec.Offset+1]
	var v uint64
	var err error
	switch {
	case param&0x80 != 0:
		var vv byte
		vv, err = dec.ReadShortInteger(nil, "")
		v = uint64(vv)
	default:
		v, err = dec.ReadLongInteger(nil, "")
	}
	dec.setPduField(reflectedPdu, hdr, v, setterUint64)

	return v, err
}

// ReadLongInteger func
func (dec *Decoder) ReadLongInteger(reflectedPdu *reflect.Value, hdr string) (uint64, error) {
	dec.Offset++
	size := int(dec.Data[dec.Offset])
	if size > shortLengthMax {
		return 0, fmt.Errorf("cannot encode long integer, lenght was %d but expected %d", size, shortLengthMax)
	}
	dec.Offset++
	end := dec.Offset + size
	var v uint64
	for ; dec.Offset < end; dec.Offset++ {
		v = v << 8
		v |= uint64(dec.Data[dec.Offset])
	}
	dec.Offset--
	dec.setPduField(reflectedPdu, hdr, v, setterUint64)

	return v, nil
}

//getParam reads the next parameter HeaderTo decode and returns it if it's well known
//or just decodes and discards if it's application specific, if the latter is
//the case it also returns false
func (dec *Decoder) getParam() (byte, bool, error) {
	if dec.Data[dec.Offset]&0x80 != 0 {
		return dec.Data[dec.Offset] & 0x7f, true, nil
	}
	var param, value string
	var err error
	dec.Offset--
	//Read the parameter name
	if param, err = dec.ReadString(nil, ""); err != nil {
		return 0, false, err
	}
	//Read the parameter value
	if value, err = dec.ReadString(nil, ""); err != nil {
		return 0, false, err
	}
	dec.log = dec.log + fmt.Sprintf("Ignoring application header: %#x: %s", param, value)
	return 0, false, nil
}

// skipFieldValue func
func (dec *Decoder) skipFieldValue() error {
	switch {
	case dec.Data[dec.Offset+1] < lengthQuote:
		l, err := dec.ReadByte(nil, "")
		if err != nil {
			return err
		}
		length := int(l)
		if dec.Offset+length >= len(dec.Data) {
			return fmt.Errorf("Bad field value length")
		}
		dec.Offset += length
		return nil
	case dec.Data[dec.Offset+1] == lengthQuote:
		dec.Offset++
		// TODO These tests should be done in basic read functions
		if dec.Offset+1 >= len(dec.Data) {
			return fmt.Errorf("Bad uintvar")
		}
		l, err := dec.ReadUintVar(nil, "")
		if err != nil {
			return err
		}
		length := int(l)
		if dec.Offset+length >= len(dec.Data) {
			return fmt.Errorf("Bad field value length")
		}
		dec.Offset += length
		return nil
	case dec.Data[dec.Offset+1] <= textMax:
		_, err := dec.ReadString(nil, "")
		return err
	}
	// case dec.Data[dec.Offset + 1] > textMax
	_, err := dec.ReadShortInteger(nil, "")
	return err
}

// Decode func
func (dec *Decoder) Decode(pdu Reader) (err error) {
	reflectedPdu := reflect.ValueOf(pdu).Elem()
	moreHdrToRead := true
	//fmt.Printf("len data: %d, data: %x\n", len(dec.Data), dec.Data)
	for ; (dec.Offset < len(dec.Data)) && moreHdrToRead; dec.Offset++ {
		//fmt.Printf("offset %d, value: %x\n", dec.Offset, dec.Data[dec.Offset])
		err = nil
		param, needsDecoding, err := dec.getParam()
		if err != nil {
			return err
		} else if !needsDecoding {
			continue
		}
		switch param {
		case HeaderXMmsMessageType:
			dec.Offset++
			expectedType := byte(reflectedPdu.FieldByName("Type").Uint())
			parsedType := dec.Data[dec.Offset]
			//Unknown message types will be discarded. OMA-WAP-MMS-ENC-v1.1 section 7.2.16
			if parsedType != expectedType {
				err = fmt.Errorf("Expected message type %x got %x", expectedType, parsedType)
			}
		case HeaderFrom:
			dec.Offset++
			size := int(dec.Data[dec.Offset])
			valStart := dec.Offset
			dec.Offset++
			token := dec.Data[dec.Offset]
			switch token {
			case tokenInsertAddress:
				break
			case tokenAddressPresent:
				// TODO add check for /TYPE=PLMN
				_, err = dec.ReadEncodedString(&reflectedPdu, "From")
				if valStart+size != dec.Offset {
					err = fmt.Errorf("From field length is %d but expected size is %d",
						dec.Offset-valStart, size)
				}
			default:
				err = fmt.Errorf("Unhandled token address in HeaderFrom field %x", token)
			}
		case HeaderXMmsExpiry:
			dec.Offset++
			size := int(dec.Data[dec.Offset])
			dec.Offset++
			token := dec.Data[dec.Offset]
			dec.Offset++
			var val uint
			endOffset := dec.Offset + size - 2
			for ; dec.Offset < endOffset; dec.Offset++ {
				val = (val << 8) | uint(dec.Data[dec.Offset])
			}
			// TODO add switch case for token
			dec.log = dec.log + fmt.Sprintf("Expiry token: %x\n", token)
			reflectedPdu.FieldByName("Expiry").SetUint(uint64(val))
			dec.log = dec.log + fmt.Sprintf("Message Expiry %d, %x\n", val, dec.Data[dec.Offset])
		case HeaderXMmsTransactionID:
			_, err = dec.ReadString(&reflectedPdu, "TransactionID")
		case HeaderContentType:
			ctMember := reflectedPdu.FieldByName("Content")
			if err = dec.ReadAttachment(&ctMember); err != nil {
				return err
			}
			//application/vnd.wap.multipart.related and others
			if ctMember.FieldByName("MediaType").String() != "text/plain" {
				err = dec.ReadAttachmentParts(&reflectedPdu)
			} else {
				dec.Offset++
				_, err = dec.ReadBoundedBytes(&reflectedPdu, "Data", len(dec.Data))
			}
			moreHdrToRead = false
		case HeaderXMmsContentLocation:
			_, err = dec.ReadString(&reflectedPdu, "ContentLocation")
			moreHdrToRead = false
		case HeaderMessageID:
			_, err = dec.ReadString(&reflectedPdu, "MessageId")
		case HeaderSubject:
			_, err = dec.ReadEncodedString(&reflectedPdu, "Subject")
		case HeaderTo:
			err = dec.ReadTo(&reflectedPdu)
		case HeaderCC:
			_, err = dec.ReadEncodedString(&reflectedPdu, "Cc")
		case HeaderXMmsReplyChargingID:
			_, err = dec.ReadString(&reflectedPdu, "ReplyChargingId")
		case HeaderXMmsRetrieveText:
			_, err = dec.ReadString(&reflectedPdu, "RetrieveText")
		case HeaderXMmsMmsVersion:
			// TODO This should be ReadShortInteger instead, but we read it
			// as a byte because we are not properly encoding the version
			// either, as we are using the raw value there. To fix this we
			// need HeaderTo change the encoder and the MMS_MESSAGE_VERSION_1_X
			// constants.
			_, err = dec.ReadByte(&reflectedPdu, "Version")
		case HeaderXMmsMessageClass:
			//TODO implement Token text form
			_, err = dec.ReadByte(&reflectedPdu, "Class")
		case HeaderXMmsReplyCharging:
			_, err = dec.ReadByte(&reflectedPdu, "ReplyCharging")
		case HeaderXMmsReplyChargingDeadline:
			_, err = dec.ReadByte(&reflectedPdu, "ReplyChargingDeadLine")
		case HeaderxMmsPriority:
			_, err = dec.ReadByte(&reflectedPdu, "Priority")
		case HeaderXMmsRetrieveStatus:
			_, err = dec.ReadByte(&reflectedPdu, "RetrieveStatus")
		case HeaderXMmsResponseStatus:
			_, err = dec.ReadByte(&reflectedPdu, "ResponseStatus")
		case HeaderXMmsResponseText:
			_, err = dec.ReadString(&reflectedPdu, "ResponseText")
		case HeaderXMmsDeliveryReport:
			_, err = dec.ReadByte(&reflectedPdu, "DeliveryReport")
		case HeaderXMmsReadReport:
			_, err = dec.ReadByte(&reflectedPdu, "ReadReport")
		case HeaderXMmsMessageSize:
			_, err = dec.ReadLongInteger(&reflectedPdu, "Size")
		case HeaderDate:
			_, err = dec.ReadLongInteger(&reflectedPdu, "Date")
		default:
			log.Printf("Skipping unrecognized header 0x%02x", param)
			err = dec.skipFieldValue()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// GetLog func
func (dec *Decoder) GetLog() string {
	return dec.log
}
