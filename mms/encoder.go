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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
)

// Encoder struct
type Encoder struct {
	w   io.Writer
	log string
}

// NewEncoder create a new instance of Encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode HeaderFrom a writer
func (enc *Encoder) Encode(pdu Writer) error {
	rPdu := reflect.ValueOf(pdu).Elem()

	//The order of the following fields doesn't matter much
	typeOfPdu := rPdu.Type()
	var err error
	for i := 0; i < rPdu.NumField(); i++ {
		fieldName := typeOfPdu.Field(i).Name
		encodeTag := typeOfPdu.Field(i).Tag.Get("encode")
		f := rPdu.Field(i)

		if encodeTag == "no" {
			continue
		}
		switch f.Kind() {
		case reflect.Uint:
		case reflect.Uint8:
			enc.log = enc.log + fmt.Sprintf("%s: %d %#x\n", fieldName, f.Uint(), f.Uint())
		case reflect.Bool:
			enc.log = enc.log + fmt.Sprintf(fieldName, f.Bool())
		default:
			enc.log = enc.log + fmt.Sprintf(fieldName, f)
		}

		switch fieldName {
		case "Type":
			err = enc.writeByteParam(HeaderXMmsMessageType, byte(f.Uint()))
		case "Version":
			err = enc.writeByteParam(HeaderXMmsMmsVersion, byte(f.Uint()))
		case "TransactionID":
			err = enc.writeStringParam(HeaderXMmsTransactionID, f.String())
		case "Status":
			err = enc.writeByteParam(HeaderXMmsStatus, byte(f.Uint()))
		case "From":
			err = enc.writeFrom()
		case "Name":
			err = enc.writeStringParam(wspParameterTypeNameDefunct, f.String())
		case "Start":
			err = enc.writeStringParam(wspParameterTypeStartDefunct, f.String())
		case "To":
			for i := 0; i < f.Len(); i++ {
				err = enc.writeStringParam(HeaderTo, f.Index(i).String())
				if err != nil {
					break
				}
			}
		case "ContentType":
			// if there is a ContentType there has HeaderTo be content
			if mSendReq, ok := pdu.(*MSendReq); ok {
				if err := enc.setParam(HeaderContentType); err != nil {
					return err
				}
				if err = enc.writeContentType(mSendReq.ContentType, mSendReq.ContentTypeStart, mSendReq.ContentTypeType, ""); err != nil {
					return err
				}
				err = enc.writeAttachments(mSendReq.Attachments)
			} else {
				err = errors.New("unhandled content type")
			}
		case "MediaType":
			if a, ok := pdu.(*Attachment); ok {
				if err = enc.writeContentType(a.MediaType, "", "", a.Name); err != nil {
					return err
				}
			} else {
				if err = enc.writeMediaType(f.String()); err != nil {
					return err
				}
			}
		case "Charset":
			//TODO
			err = enc.writeCharset(f.String())
		case "ContentLocation":
			err = enc.writeStringParam(mmsPartContentLocation, f.String())
		case "ContentID":
			err = enc.writeQuotedStringParam(mmsPartContentID, f.String())
		case "Date":
			dateTime := f.Uint()
			if dateTime > 0 {
				err = enc.writeLongIntegerParam(HeaderDate, dateTime)
			}
		case "Class":
			err = enc.writeByteParam(HeaderXMmsMessageClass, byte(f.Uint()))
		case "ReportAllowed":
			err = enc.writeByteParam(HeaderXMmsReportAllowed, byte(f.Uint()))
		case "DeliveryReport":
			err = enc.writeByteParam(HeaderXMmsDeliveryReport, byte(f.Uint()))
		case "ReadReport":
			err = enc.writeByteParam(HeaderXMmsReadReport, byte(f.Uint()))
		case "Expiry":
			expiry := f.Uint()
			if expiry > 0 {
				err = enc.writeRelativeExpiry(expiry)
			}
		default:
			if encodeTag == "optional" {
				log.Printf("Unhandled optional field %s", fieldName)
			} else {
				return fmt.Errorf("missing encoding for mandatory field %s", fieldName)
			}
		}
		if err != nil {
			return fmt.Errorf("cannot encode field %s with value %s: %s ... encoded so far: %s", fieldName, f, err, enc.log)
		}
	}
	return nil
}

// setParam func
func (enc *Encoder) setParam(param byte) error {
	return enc.writeByte(param | 0x80)
}

// encodeAttachment encode an attachment struct
func encodeAttachment(attachment *Attachment) ([]byte, error) {
	var outBytes bytes.Buffer
	enc := NewEncoder(&outBytes)
	if err := enc.Encode(attachment); err != nil {
		return []byte{}, err
	}
	return outBytes.Bytes(), nil
}

// writeAttachments write an attachment
func (enc *Encoder) writeAttachments(attachments []*Attachment) error {
	// Write the number of parts
	if err := enc.writeUintVar(uint64(len(attachments))); err != nil {
		return err
	}

	for i := range attachments {
		var attachmentHeader []byte
		b, err := encodeAttachment(attachments[i])
		if err != nil {
			return err
		}
		attachmentHeader = b

		// headers length
		headerLength := uint64(len(attachmentHeader))
		if err := enc.writeUintVar(headerLength); err != nil {
			return err
		}
		// data length
		dataLength := uint64(len(attachments[i].Data))
		if err := enc.writeUintVar(dataLength); err != nil {
			return err
		}
		if err := enc.writeBytes(attachmentHeader, int(headerLength)); err != nil {
			return err
		}
		if err := enc.writeBytes(attachments[i].Data, int(dataLength)); err != nil {
			return err
		}
	}
	return nil
}

// writeCharset func
func (enc *Encoder) writeCharset(charset string) error {
	if charset == "" {
		return nil
	}
	charsetCode := uint64(anyCharset)
	for k, v := range CHARSETS {
		if v == charset {
			charsetCode = k
		}
	}
	return enc.writeIntegerParam(wspParameterTypeCharset, charsetCode)
}

// writeLength func
func (enc *Encoder) writeLength(length uint64) error {
	if length <= shortLengthMax {
		return enc.writeByte(byte(length))
	}
	if err := enc.writeByte(lengthQuote); err != nil {
		return err
	}
	return enc.writeUintVar(length)
}

// encodeContentType func
func encodeContentType(media string) (uint64, error) {
	var mt int
	for mt = range contentTypes {
		if contentTypes[mt] == media {
			return uint64(mt), nil
		}
	}
	return 0, errors.New("cannot binary encode media")
}

// writeContentType func
func (enc *Encoder) writeContentType(media, start, ctype, name string) error {
	if start == "" && ctype == "" && name == "" {
		return enc.writeMediaType(media)
	}

	var contentType []byte
	if start != "" {
		contentType = append(contentType, wspParameterTypeStartDefunct|shortFilter)
		contentType = append(contentType, []byte(start)...)
		contentType = append(contentType, 0)
	}
	if ctype != "" {
		contentType = append(contentType, wspParameterTypeContentType|shortFilter)
		contentType = append(contentType, []byte(ctype)...)
		contentType = append(contentType, 0)
	}
	if name != "" {
		contentType = append(contentType, wspParameterTypeNameDefunct|shortFilter)
		contentType = append(contentType, []byte(name)...)
		contentType = append(contentType, 0)
	}

	if mt, err := encodeContentType(media); err == nil {
		// +1 for mt
		length := uint64(len(contentType) + 1)
		if err := enc.writeLength(length); err != nil {
			return err
		}
		if err := enc.writeInteger(mt); err != nil {
			return err
		}
	} else {
		mediaB := []byte(media)
		mediaB = append(mediaB, 0)
		contentType = append(mediaB, contentType...)
		length := uint64(len(contentType))
		if err := enc.writeLength(length); err != nil {
			return err
		}
	}
	return enc.writeBytes(contentType, len(contentType))
}

// writeMediaType func
func (enc *Encoder) writeMediaType(media string) error {
	if mt, err := encodeContentType(media); err == nil {
		return enc.writeInteger(mt)
	}

	// +1 is the byte{0}
	if err := enc.writeByte(byte(len(media) + 1)); err != nil {
		return err
	}
	return enc.writeString(media)
}

// writeRelativeExpiry func
func (enc *Encoder) writeRelativeExpiry(expiry uint64) error {
	if err := enc.setParam(HeaderXMmsExpiry); err != nil {
		return err
	}
	encodedLong := encodeLong(expiry)

	var b []byte
	// +1 for the token, +1 for the len of long
	b = append(b, byte(len(encodedLong)+2))
	b = append(b, ExpiryTokenRelative)
	b = append(b, byte(len(encodedLong)))
	b = append(b, encodedLong...)

	return enc.writeBytes(b, len(b))
}

// writeLongIntegerParam func
func (enc *Encoder) writeLongIntegerParam(param byte, i uint64) error {
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeLongInteger(i)
}

// writeIntegerParam func
func (enc *Encoder) writeIntegerParam(param byte, i uint64) error {
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeInteger(i)
}

// writeQuotedStringParam func
func (enc *Encoder) writeQuotedStringParam(param byte, s string) error {
	if s == "" {
		enc.log = enc.log + "Skipping empty string\n"
	}
	if err := enc.setParam(param); err != nil {
		return err
	}
	if err := enc.writeByte(stringQuote); err != nil {
		return err
	}
	return enc.writeString(s)
}

// writeStringParam func
func (enc *Encoder) writeStringParam(param byte, s string) error {
	if s == "" {
		enc.log = enc.log + "Skipping empty string\n"
		return nil
	}
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeString(s)
}

// writeByteParam func
func (enc *Encoder) writeByteParam(param byte, b byte) error {
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeByte(b)
}

// writeFrom func
func (enc *Encoder) writeFrom() error {
	if err := enc.setParam(HeaderFrom); err != nil {
		return err
	}
	if err := enc.writeByte(1); err != nil {
		return err
	}
	return enc.writeByte(tokenInsertAddress)
}

// writeString func
func (enc *Encoder) writeString(s string) error {
	bytes := []byte(s)
	bytes = append(bytes, 0)
	_, err := enc.w.Write(bytes)
	return err
}

// writeBytes func
func (enc *Encoder) writeBytes(b []byte, count int) error {
	if n, err := enc.w.Write(b); n != count {
		return fmt.Errorf("expected HeaderTo write %d byte[s] but wrote %d", count, n)
	} else if err != nil {
		return err
	}
	return nil
}

// writeByte func
func (enc *Encoder) writeByte(b byte) error {
	return enc.writeBytes([]byte{b}, 1)
}

// writeShort encodes i according HeaderTo the Basic Rules described in section
// 8.4.2.2 of WAP-230-WSP-20010705-a.
//
// Integers in range 0-127 (< 0x80) shall be encoded as a one octet value
// with the most significant bit set HeaderTo one (1xxx xxxx == |0x80) and with
// the value in the remaining least significant bits.
func (enc *Encoder) writeShortInteger(i uint64) error {
	return enc.writeByte(byte(i | 0x80))
}

// writeLongInteger encodes i according HeaderTo the Basic Rules described in section
// 8.4.2.2 of WAP-230-WSP-20010705-a.
//
// Long-integer = Short-length Multi-octet-integer
// The Short-length indicates the length of the Multi-octet-integer
//
// Multi-octet-integer = 1*30 OCTET
// The content octets shall be an unsigned integer value
// with the most significant octet encoded first (big-endian representation).
// The minimum number of octets must be used HeaderTo encode the value.
func (enc *Encoder) writeLongInteger(i uint64) error {
	encodedLong := encodeLong(i)
	encLength := uint64(len(encodedLong))
	if encLength > shortLengthMax {
		return fmt.Errorf("cannot encode long integer, lenght was %d but expected %d", encLength, shortLengthMax)
	}
	if err := enc.writeByte(byte(encLength)); err != nil {
		return err
	}

	return enc.writeBytes(encodedLong, len(encodedLong))
}

func encodeLong(i uint64) (encodedLong []byte) {
	for i > 0 {
		b := byte(0xff & i)
		encodedLong = append([]byte{b}, encodedLong...)
		i = i >> 8
	}
	return encodedLong
}

// writeInteger encodes i according HeaderTo the Basic Rules described in section
// 8.4.2.2 of WAP-230-WSP-20010705-a.
//
// It encodes as a Short-integer when i < 128 (=0x80) or as a Long-Integer
// otherwise
func (enc *Encoder) writeInteger(i uint64) error {
	if i < 0x80 {
		return enc.writeShortInteger(i)
	}
	return enc.writeLongInteger(i)
}

// writeUintVar encodes v according HeaderTo section 8.1.2 and the Basic Rules
// described in section 8.4.2.2 of WAP-230-WSP-20010705-a.
//
// To encode a large unsigned integer, split it into 7-bit (0x7f) fragments
// and place them in the payloads of multiple octets. The most significant
// bits are placed in the first octets with the least significant bits ending
// up in the last octet. All octets MUST set the Continue bit HeaderTo 1 (|0x80)
// except the last octet, which MUST set the Continue bit HeaderTo 0.
//
// The unsigned integer MUST be encoded in the smallest encoding possible.
// In other words, the encoded value MUST NOT start with an octet with the
// value 0x80.
func (enc *Encoder) writeUintVar(v uint64) error {
	uintVar := []byte{byte(v & 0x7f)}
	v = v >> 7
	for v > 0 {
		uintVar = append([]byte{byte(0x80 | (v & 0x7f))}, uintVar...)
		v = v >> 7
	}
	return enc.writeBytes(uintVar, len(uintVar))
}
