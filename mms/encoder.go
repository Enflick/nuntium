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

type MMSEncoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *MMSEncoder {
	return &MMSEncoder{w}
}

func (enc *MMSEncoder) Encode(pdu MMSWriter) error {
	rPdu := reflect.ValueOf(pdu).Elem()

	//The order of the following fields doens't matter much
	typeOfPdu := rPdu.Type()
	var err error
	for i := 0; i < rPdu.NumField(); i++ {
		fieldName := typeOfPdu.Field(i).Name
		encodeTag := typeOfPdu.Field(i).Tag.Get("encode")
		f := rPdu.Field(i)

		if encodeTag == "no" || encodeTag == "optional" {
			continue
		}
		switch f.Kind() {
		case reflect.Uint:
		case reflect.Uint8:
			fmt.Printf("%s: %d %#x\n", fieldName, f.Uint(), f.Uint())
		case reflect.Bool:
			fmt.Println(fieldName, f.Bool())
		default:
			fmt.Println(fieldName, f)
		}

		switch fieldName {
		case "Type":
			err = enc.writeByteParam(X_MMS_MESSAGE_TYPE, byte(f.Uint()))
		case "Version":
			err = enc.writeByteParam(X_MMS_MMS_VERSION, byte(f.Uint()))
		case "TransactionId":
			err = enc.writeStringParam(X_MMS_TRANSACTION_ID, f.String())
		case "Status":
			err = enc.writeByteParam(X_MMS_STATUS, byte(f.Uint()))
		case "ReportAllowed":
			err = enc.writeReportAllowedParam(f.Bool())
		case "From":
			err = enc.writeFrom()
		case "Name":
			err = enc.writeStringParam(WSP_PARAMETER_TYPE_NAME, f.String())
		case "Start":
			err = enc.writeStringParam(WSP_PARAMETER_TYPE_START, f.String())
		case "To":
			err = enc.writeStringParam(TO, f.String())
		case "ContentType":
			// if there is a ContentType there has to be content
			if mSendReq, ok := pdu.(*MSendReq); ok {
				if err = enc.writeMediaType(mSendReq.ContentType); err != nil {
					return err
				}
				err = enc.writeAttachments(mSendReq.Attachments)
			} else {
				err = errors.New("unhandled content type")
			}
		case "MediaType":
			if err = enc.writeMediaType(f.String()); err != nil {
				return err
			}
		case "Charset":
			//TODO
			err = enc.writeCharset(f.String())
		case "ContentLocation":
			err = enc.writeStringParam(MMS_PART_CONTENT_LOCATION, f.String())
		case "ContentId":
			err = enc.writeStringParam(MMS_PART_CONTENT_ID, f.String())
		default:
			if encodeTag == "optional" {
				log.Printf("Unhandled optional field %s", fieldName)
			} else {
				panic(fmt.Sprintf("missing encoding for mandatory field %s", fieldName))
			}
		}
		if err != nil {
			return fmt.Errorf("cannot encode field %s with value %s: %s", fieldName, f, err)
		}
	}
	return nil
}

func (enc *MMSEncoder) setParam(param byte) error {
	return enc.writeByte(param | 0x80)
}

func encodeAttachment(attachment *Attachment) ([]byte, error) {
	var outBytes bytes.Buffer
	enc := NewEncoder(&outBytes)
	if err := enc.Encode(attachment); err != nil {
		return []byte{}, err
	}
	return outBytes.Bytes(), nil
}

func (enc *MMSEncoder) writeAttachments(attachments []*Attachment) error {
	// Write the number of parts
	if err := enc.writeUintVar(uint64(len(attachments))); err != nil {
		return err
	}

	var attachmentHeaders [][]byte
	for i := range attachments {
		if b, err := encodeAttachment(attachments[i]); err != nil {
			return err
		} else {
			attachmentHeaders = append(attachmentHeaders, b)
		}

		// attachments index matches attachmentHeaders
		// TODO use a map
		for i := range attachmentHeaders {
			// headers length
			headerLength := uint64(len(attachmentHeaders[i]))
			if err := enc.writeUintVar(headerLength); err != nil {
				return err
			}
			// data length
			dataLength := uint64(len(attachments[i].Data))
			if err := enc.writeUintVar(dataLength); err != nil {
				return err
			}
			if err := enc.writeBytes(attachmentHeaders[i], int(headerLength)); err != nil {
				return err
			}
			if err := enc.writeBytes(attachments[i].Data, int(dataLength)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (enc *MMSEncoder) writeCharset(charset string) error {
	if charset == "" {
		return nil
	}
	charsetCode := uint64(ANY_CHARSET)
	for k, v := range CHARSETS {
		if v == charset {
			charsetCode = k
		}
	}
	return enc.writeIntegerParam(WSP_PARAMETER_TYPE_CHARSET, charsetCode)
}

func (enc *MMSEncoder) writeLength(length uint64) error {
	if length < SHORT_LENGTH_MAX {
		return enc.writeShortInteger(length)
	} else {
		if err := enc.writeByte(LENGTH_QUOTE); err != nil {
			return err
		}
		return enc.writeUintVar(length)
	}
}

func (enc *MMSEncoder) writeMediaType(media string) error {
	if err := enc.setParam(WSP_PARAMETER_TYPE_CONTENT_TYPE); err != nil {
		return err
	}
	var mt int
	for mt = range CONTENT_TYPES {
		if CONTENT_TYPES[mt] == media {
			fmt.Println("Media Type is ", media, "in", mt, "which is the same as", CONTENT_TYPES[mt])
			return enc.writeInteger(uint64(mt))
		}
	}

	if err := enc.writeLength(uint64(len(media))); err != nil {
		return err
	}
	return enc.writeString(media)
}

func (enc *MMSEncoder) writeIntegerParam(param byte, i uint64) error {
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeInteger(i)
}

func (enc *MMSEncoder) writeStringParam(param byte, s string) error {
	if s == "" {
		log.Print("Skipping empty string")
	}
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeString(s)
}

func (enc *MMSEncoder) writeByteParam(param byte, b byte) error {
	if err := enc.setParam(param); err != nil {
		return err
	}
	return enc.writeByte(b)
}

func (enc *MMSEncoder) writeReportAllowedParam(reportAllowed bool) error {
	if err := enc.setParam(X_MMS_REPORT_ALLOWED); err != nil {
		return err
	}
	var b byte
	if reportAllowed {
		b = REPORT_ALLOWED_YES
	} else {
		b = REPORT_ALLOWED_NO
	}
	return enc.writeByte(b)
}

func (enc *MMSEncoder) writeFrom() error {
	if err := enc.setParam(FROM); err != nil {
		return err
	}
	if err := enc.writeLength(1); err != nil {
		return err
	}
	return enc.writeByte(TOKEN_INSERT_ADDRESS)
}

func (enc *MMSEncoder) writeString(s string) error {
	bytes := []byte(s)
	bytes = append(bytes, 0)
	_, err := enc.w.Write(bytes)
	return err
}

func (enc *MMSEncoder) writeBytes(b []byte, count int) error {
	if n, err := enc.w.Write(b); n != count {
		return fmt.Errorf("expected to write %d byte[s] but wrote %d", count, n)
	} else if err != nil {
		return err
	}
	return nil
}

func (enc *MMSEncoder) writeByte(b byte) error {
	return enc.writeBytes([]byte{b}, 1)
}

func (enc *MMSEncoder) writeShortInteger(i uint64) error {
	return enc.writeByte(byte(i | 0x80))
}

func (enc *MMSEncoder) writeLongInteger(i uint64) error {
	long := i
	var encodedLong []byte
	for long > 0 {
		b := byte(0xff & long)
		encodedLong = append(encodedLong, b)
		long = long >> 8
	}

	encLength := uint64(len(encodedLong))
	if encLength > SHORT_LENGTH_MAX {
		return fmt.Errorf("cannot encode long integer, lenght was %d but expected %d", encLength, SHORT_LENGTH_MAX)
	}
	if err := enc.writeShortInteger(encLength); err != nil {
		return err
	}
	return enc.writeBytes(encodedLong, len(encodedLong))
}

func (enc *MMSEncoder) writeInteger(i uint64) error {
	if i&0x80 < 0x80 {
		return enc.writeShortInteger(i)
	} else {
		return enc.writeLongInteger(i)
	}
	return nil
}

func (enc *MMSEncoder) writeUintVar(v uint64) error {
	b := byte(v)
	uintVar := []byte{b & 0x7f}
	b = b >> 7
	for b > 0 {
		uintVar = append([]byte{b & 0x7f}, uintVar...)
		b = b >> 7
	}
	return enc.writeBytes(uintVar, len(uintVar))
}