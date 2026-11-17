package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

type NoMixin struct{ ent.Schema }

func (NoMixin) Fields() []ent.Field {
	return []ent.Field{field.String("name").NotEmpty(), field.String("ok").MaxLen(10), field.Text("body")}
}

type TwoMixins struct{ ent.Schema }

func (TwoMixins) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{}, entx.GlobalBase{}} }

type Good struct{ ent.Schema }

func (Good) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{}} }
func (Good) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(32).NotEmpty(),
		// maxlen-justified: rich text bounded by CHECK octet_length <= 65536
		field.Text("notes"),
	}
}
