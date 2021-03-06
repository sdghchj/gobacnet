package gobacnet

import (
	"github.com/alexbeltran/gobacnet/encoding"
	bactype "github.com/alexbeltran/gobacnet/types"
)

func (c *client) iAm(dest bactype.Address) error {
	npdu := &bactype.NPDU{
		Version:               bactype.ProtocolVersion,
		Destination:           &dest,
		IsNetworkLayerMessage: false,
		ExpectingReply:        false,
		Priority:              bactype.Normal,
		HopCount:              bactype.DefaultHopCount,
	}
	enc := encoding.NewEncoder()
	enc.NPDU(npdu)

	//	iams := []bactype.ObjectID{bactype.ObjectID{Instance: 1, Type: 5}}
	//	enc.IAm(iams)
	_, err := c.Send(dest, npdu, enc.Bytes())
	return err
}
