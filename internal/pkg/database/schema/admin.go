package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/go-sphere/entc-extensions/entproto"
	"github.com/go-sphere/sphere/utils/idgenerator"
)

type Admin struct {
	ent.Schema
}

func (Admin) Fields() []ent.Field {
	times := DefaultTimeProtoFields([2]int{7, 8})
	return []ent.Field{
		field.Int64("id").Annotations(entproto.Field(1)).Unique().Immutable().DefaultFunc(idgenerator.NextId).Comment("用户ID"),
		field.String("username").Annotations(entproto.Field(2)).Unique().MinLen(1).Comment("用户名"),
		field.String("nickname").Annotations(entproto.Field(3)).Default("").Comment("昵称"),
		field.String("avatar").Annotations(entproto.Field(4)).Default("").Comment("头像"),
		field.String("password").Annotations(entproto.Field(5)).Comment("密码").Sensitive(),
		field.Strings("roles").Annotations(entproto.Field(6)).Default([]string{}).Comment("权限").Sensitive(),
		times[0], times[1],
	}
}
func (Admin) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

type AdminSession struct {
	ent.Schema
}

func (AdminSession) Fields() []ent.Field {
	times := DefaultTimeProtoFields([2]int{8, 9})
	return []ent.Field{
		field.Int64("id").Annotations(entproto.Field(1)).Unique().Immutable().DefaultFunc(idgenerator.NextId).Comment("会话ID"),
		field.Int64("uid").Annotations(entproto.Field(2)).Immutable().Comment("用户ID"),
		field.String("session_key").Annotations(entproto.Field(3)).Immutable().Sensitive().MaxLen(36).Comment("会话ID"),
		field.Int64("expires").Annotations(entproto.Field(4)).Immutable().DefaultFunc(TimestampDefaultFunc).Comment("过期时间"),
		field.Bool("is_revoked").Annotations(entproto.Field(5)).Default(false).Comment("是否已撤销"),
		field.String("device_info").Annotations(entproto.Field(6)).Default("").Comment("设备信息"),
		field.String("ip_address").Annotations(entproto.Field(7)).Default("").Comment("IP地址"),
		times[0], times[1],
	}
}

func (AdminSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("uid"),
	}
}

func (AdminSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}
