package gobac

type writePropertyRequest struct {
	*Request
	object     *Object
	propertyId PropertyId
}

func (s *Server) SendWritePropertyRequest(object *Object,
	propertyId PropertyId,
	propertyType DataTag,
	priority uint8,
	value interface{}) {

}
