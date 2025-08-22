/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-07-12 18:40:05
 * @LastEditTime: 2025-07-12 18:40:10
 * @LastEditors: 安知鱼
 */
package schema

import (
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent/schema/mixin"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Album holds the schema definition for the Album entity.
type Album struct {
	ent.Schema
}

// Mixin of the Album.
func (Album) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Album.
func (Album) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("id"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.String("image_url").
			MaxLen(255).
			NotEmpty().
			Comment("图片URL"),
		field.String("big_image_url").
			MaxLen(255).
			Optional().
			Comment("大图URL"),
		field.String("download_url").
			MaxLen(255).
			Optional().
			Comment("下载URL"),
		field.String("thumb_param").
			MaxLen(512).
			Optional().
			Comment("缩略图处理参数"),
		field.String("big_param").
			MaxLen(512).
			Optional().
			Comment("大图处理参数"),
		field.String("tags").
			MaxLen(255).
			Optional().
			Comment("标签，逗号分隔"),
		field.Int("view_count").
			Default(0).
			Comment("查看次数"),
		field.Int("download_count").
			Default(0).
			Comment("下载次数"),
		field.Int("width").
			Optional().
			Comment("图片宽度"),
		field.Int("height").
			Optional().
			Comment("图片高度"),
		field.Int64("file_size").
			Optional().
			Comment("文件大小（字节）"),
		field.String("format").
			MaxLen(50).
			Optional().
			Comment("图片格式"),
		field.String("aspect_ratio").
			MaxLen(50).
			Optional().
			Comment("图片宽高比"),
		field.String("file_hash").
			MaxLen(64).
			Unique(). // 文件哈希值是唯一的
			Comment("文件哈希值"),
		field.Int("display_order").
			Default(0).
			Comment("排序字段，数字越小越靠前"),
	}
}
