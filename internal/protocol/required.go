package protocol

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FillRequired sets every unset required field of msg, recursively, to its
// zero value (0, "", false, or an empty message), so a reply only has to
// spell out the fields that carry meaning. Set fields are left alone.
func FillRequired[M proto.Message](msg M) M {
	fillRequired(msg.ProtoReflect())
	return msg
}

func fillRequired(msg protoreflect.Message) {
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		switch {
		case field.Cardinality() == protoreflect.Required && field.Message() != nil:
			fillRequired(msg.Mutable(field).Message())
		case field.Cardinality() == protoreflect.Required && !msg.Has(field):
			msg.Set(field, field.Default())
		case field.Message() != nil && field.IsList():
			list := msg.Get(field).List()
			for j := 0; j < list.Len(); j++ {
				fillRequired(list.Get(j).Message())
			}
		case field.Message() != nil && msg.Has(field):
			fillRequired(msg.Mutable(field).Message())
		}
	}
}
