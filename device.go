/*Copyright (C) 2017 Alex Beltran

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to:
The Free Software Foundation, Inc.
59 Temple Place - Suite 330
Boston, MA  02111-1307, USA.

As a special exception, if other files instantiate templates or
use macros or inline functions from this file, or you compile
this file and link it with other works to produce a work based
on this file, this file does not by itself cause the resulting
work to be covered by the GNU General Public License. However
the source code for this file must still be made available in
accordance with section (3) of the GNU General Public License.

This exception does not invalidate any other reasons why a work
based on this file might be covered by the GNU General Public
License.
*/

package gobacnet

import (
	"fmt"
	"github.com/alexbeltran/gobacnet/datalink"
	"github.com/alexbeltran/gobacnet/encoding"
	bactype "github.com/alexbeltran/gobacnet/types"
	"io"
	"sync"
	"time"

	"github.com/alexbeltran/gobacnet/tsm"
	"github.com/alexbeltran/gobacnet/utsm"
	"github.com/sirupsen/logrus"
)

const mtuHeaderLength = 4
const defaultStateSize = 20
const forwardHeaderLength = 10

type Client interface {
	io.Closer
	Run()
	WhoIs(low, high int) ([]bactype.Device, error)
	Objects(dev bactype.Device) (bactype.Device, error)
	ReadProperty(dest bactype.Device, rp bactype.PropertyData) (bactype.PropertyData, error)
	ReadMultiProperty(dev bactype.Device, rp bactype.MultiplePropertyData) (bactype.MultiplePropertyData, error)
	WriteProperty(dest bactype.Device, wp bactype.PropertyData) error
	WriteMultiProperty(dev bactype.Device, wp bactype.MultiplePropertyData) error
}

type client struct {
	dataLink       datalink.DataLink
	tsm            *tsm.TSM
	utsm           *utsm.Manager
	readBufferPool sync.Pool
	log            *logrus.Logger
}

// NewClient creates a new client with the given interface and
// port.
func NewClient(dataLink datalink.DataLink, maxPDU uint16) Client {
	if maxPDU == 0 {
		maxPDU = bactype.MaxAPDU
	}
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{}
	log.SetLevel(logrus.InfoLevel)
	return &client{
		dataLink: dataLink,
		tsm:      tsm.New(defaultStateSize),
		utsm: utsm.NewManager(
			utsm.DefaultSubscriberTimeout(time.Second*time.Duration(10)),
			utsm.DefaultSubscriberLastReceivedTimeout(time.Second*time.Duration(2)),
		),
		readBufferPool: sync.Pool{New: func() interface{} {
			return make([]byte, maxPDU)
		}},
		log: log,
	}
}

func (c *client) Run() {
	var err error = nil
	for err == nil {
		b := c.readBufferPool.Get().([]byte)
		var addr *bactype.Address
		var n int
		addr, n, err = c.dataLink.Receive(b)
		if err != nil {
			continue
		}
		go c.handleMsg(addr, b[:n])
	}
}

func (c *client) handleMsg(src *bactype.Address, b []byte) {
	var header bactype.BVLC
	var npdu bactype.NPDU
	var apdu bactype.APDU

	dec := encoding.NewDecoder(b)
	err := dec.BVLC(&header)
	if err != nil {
		c.log.Error(err)
		return
	}

	if header.Function == bactype.BacFuncBroadcast || header.Function == bactype.BacFuncUnicast || header.Function == bactype.BacFuncForwardedNPDU {
		// Remove the header information
		b = b[mtuHeaderLength:]
		err = dec.NPDU(&npdu)
		if err != nil {
			return
		}

		if npdu.IsNetworkLayerMessage {
			c.log.Debug("Ignored Network Layer Message")
			return
		}

		// We want to keep the APDU intact so we will get a snapshot before decoding
		// further
		send := dec.Bytes()
		err = dec.APDU(&apdu)
		if err != nil {
			c.log.Errorf("Issue decoding APDU: %v", err)
			return
		}

		switch apdu.DataType {
		case bactype.UnconfirmedServiceRequest:
			if apdu.UnconfirmedService == bactype.ServiceUnconfirmedIAm {
				c.log.Debug("Received IAm Message")
				dec = encoding.NewDecoder(apdu.RawData)
				var iam bactype.IAm

				err = dec.IAm(&iam)

				iam.Addr = *src
				if err != nil {
					c.log.Error(err)
					return
				}
				c.utsm.Publish(int(iam.ID.Instance), iam)
			} else if apdu.UnconfirmedService == bactype.ServiceUnconfirmedWhoIs {
				dec := encoding.NewDecoder(apdu.RawData)
				var low, high int32
				dec.WhoIs(&low, &high)
				// For now we are going to ignore who is request.
				//log.WithFields(log.Fields{"low": low, "high": high}).Debug("WHO IS Request")
			} else {
				c.log.Errorf("Unconfirmed: %d %v", apdu.UnconfirmedService, apdu.RawData)
			}
		case bactype.SimpleAck:
			c.log.Debug("Received Simple Ack")
			err := c.tsm.Send(int(apdu.InvokeId), send)
			if err != nil {
				return
			}
		case bactype.ComplexAck:
			c.log.Debug("Received Complex Ack")
			err := c.tsm.Send(int(apdu.InvokeId), send)
			if err != nil {
				return
			}
		case bactype.ConfirmedServiceRequest:
			c.log.Debug("Received  Confirmed Service Request")
			err := c.tsm.Send(int(apdu.InvokeId), send)
			if err != nil {
				return
			}
		case bactype.Error:
			err := fmt.Errorf("error class %d code %d", apdu.Error.Class, apdu.Error.Code)
			err = c.tsm.Send(int(apdu.InvokeId), err)
			if err != nil {
				c.log.Debugf("unable to Send error to %d: %v", apdu.InvokeId, err)
			}
		default:
			// Ignore it
			//log.WithFields(log.Fields{"raw": b}).Debug("An ignored packet went through")
		}
	}

	if header.Function == bactype.BacFuncForwardedNPDU {
		// Right now we are ignoring the NPDU data that is stored in the packet. Eventually
		// we will need to check it for any additional information we can gleam.
		// NDPU has source
		b = b[forwardHeaderLength:]
		c.log.Debug("Ignored NDPU Forwarded")
	}
}

// Send transfers the raw apdu byte slice to the destination address.
func (c *client) Send(dest bactype.Address, npdu *bactype.NPDU, data []byte) (int, error) {
	var header bactype.BVLC

	// Set packet type
	header.Type = bactype.BVLCTypeBacnetIP

	if dest.IsBroadcast() || dest.IsSubBroadcast() {
		// SET BROADCAST FLAG
		header.Function = bactype.BacFuncBroadcast
	} else {
		// SET UNICAST FLAG
		header.Function = bactype.BacFuncUnicast
	}
	header.Length = uint16(mtuHeaderLength + len(data))
	header.Data = data
	e := encoding.NewEncoder()
	err := e.BVLC(header)
	if err != nil {
		return 0, err
	}

	// use default udp type, src = local address (nil)
	return c.dataLink.Send(e.Bytes(), npdu, &dest)
}

// Close free resources for the client. Always call this function when using NewClient
func (c *client) Close() error {
	if c.dataLink != nil {
		c.dataLink.Close()
	}
	if f, ok := c.log.Out.(io.Closer); ok {
		return f.Close()
	}
	return nil
}
