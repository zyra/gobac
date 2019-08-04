package gobac

type PropertyValue struct {
	Type  DataTag
	Value interface{}
}

func (p *PropertyValue) ReadAsObject() Object {
	obj := Object{}

	if v, ok := p.Value.(ObjectIdValue); ok {
		obj.Type = v.Type
		obj.Instance = v.Instance
	}

	return obj
}
