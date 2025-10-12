/*
 * @Description: 相册分类 Schema
 * @Author: 安知鱼
 * @Date: 2025-10-12
 */
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// AlbumCategory holds the schema definition for the AlbumCategory entity.
type AlbumCategory struct {
	ent.Schema
}

// Fields of the AlbumCategory.
func (AlbumCategory) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("id"),
		field.String("name").
			Comment("分类名称").
			Unique().
			NotEmpty(),
		field.String("description").
			Comment("分类描述").
			Optional(),
		field.Int("display_order").
			Comment("显示顺序").
			Default(0),
	}
}

// Edges of the AlbumCategory.
func (AlbumCategory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("albums", Album.Type),
	}
}
