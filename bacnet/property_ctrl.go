package bacnet

import "github.com/zyra/gobac/bacnet/types"

type PropertyController types.Property

func (p PropertyController) RawValue() *types.Property {
	prop := types.Property(p)
	return &prop
}

func (p *PropertyController) GetValue(server *Server) error {
	deviceAddress := p.IPAddress
	objectType := p.ObjectId.Type
	objectInstance := p.ObjectId.Instance
	if prop, err := server.ReadProperty(deviceAddress, objectType, objectInstance, p.ID); err != nil {
		return err
	} else {
		p.Values = prop
	}
	return nil
}

func (p *PropertyController) SetValue(server *Server, dataType DataTag, value interface{}) error {
	deviceAddress := p.IPAddress
	objectType := p.ObjectId.Type
	objectInstance := p.ObjectId.Instance
	return server.WriteProperty(deviceAddress, objectType, objectInstance, p.ID, dataType, 7, value)
}
