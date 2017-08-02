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
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/alexbeltran/gobacnet/encoding"
	"github.com/alexbeltran/gobacnet/types"
)

const interfaceName = "eth1"

// TestMain are general test
func TestMain(t *testing.T) {
	c, err := NewClient(interfaceName, DefaultPort)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	d, err := NewClient("pizzainterfacenotreal", DefaultPort)
	d.Close()
	if err == nil {
		t.Fatal("Successfully passed a false interface.")
	}
}
func TestGetBroadcast(t *testing.T) {
	failTest := func(addr string) {
		_, err := getBroadcast(addr)
		if err == nil {
			t.Fatalf("%s is not a valid parameter, but it did not gracefully crash", addr)
		}
	}

	failTest("frog")
	failTest("frog/dog")
	failTest("frog/24")
	failTest("16.18.dog/32")

	s, err := getBroadcast("192.168.23.1/24")
	if err != nil {
		t.Fatal(err)
	}
	correct := "192.168.23.255"
	if s.String() != correct {
		t.Fatalf("%s is incorrect. It should be %s", s.String(), correct)
	}
}

func TestReadPropertyService(t *testing.T) {
	c, err := NewClient(interfaceName, DefaultPort+1)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	time.Sleep(time.Duration(1) * time.Second)

	var mac []byte
	var adr []byte
	json.Unmarshal([]byte("\"ChQAzLrA\""), &mac)
	json.Unmarshal([]byte("\"HQ==\""), &adr)
	log.Println(mac)
	dest := types.Address{
		Net:    2428,
		Len:    1,
		MacLen: 6,
		Mac:    mac,
		Adr:    adr,
	}
	read := types.ReadPropertyData{
		Object: types.Object{
			ID: types.ObjectID{
				Type:     0,
				Instance: 1,
			},
			Properties: []types.Property{
				types.Property{
					Type:       85, // Present value
					ArrayIndex: 0xFFFFFFFF,
				},
			},
		},
	}
	resp, err := c.ReadProperty(&dest, read)
	if err != nil {
		t.Fatal(err)
	}
	dec := encoding.NewDecoder(resp.Object.Properties[0].Data)
	out, err := dec.AppData()
	log.Printf("Out Value 1: %v", out)
	log.Printf("%v", read.Object.Properties[0].Data)

	/*
		read.ObjectProperty = 76
		read.ObjectInstance = 242829
		read.ObjectType = 8
		//	read.ArrayIndex = 0
		resp, err = c.ReadProperty(&dest, read)
		if err != nil {
			t.Fatal(err)
		}

		dec = encoding.NewDecoder(resp.ApplicationData)
		out, err = dec.AppData()
		if err != nil {
			t.Fatal(err)
		}
		log.Printf("Out Value 2: %v", out)
		log.Printf("Raw: %v", resp.ApplicationData)
	*/
}
func TestMac(t *testing.T) {
	var mac []byte
	json.Unmarshal([]byte("\"ChQAzLrA\""), &mac)
	l := len(mac)
	p := uint16(mac[l-1])<<8 | uint16(mac[l-1])
	log.Printf("%d", p)
}

func TestWhoIs(t *testing.T) {
	time.Sleep(time.Duration(1) * time.Second)
	c, err := NewClient(interfaceName, DefaultPort)
	if err != nil {
		t.Fatal(err)
	}
	err = c.WhoIs(242800, 242900)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(30) * time.Second)
	c.Close()
}
