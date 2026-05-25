package generate

import (
	"testing"

	"gorm.io/gen/field"
	"gorm.io/gen/internal/model"
)

func TestGetFieldsAppliesModifyOptionsToCreatedRelations(t *testing.T) {
	conf := (&model.Config{
		ModelPkg: "model",
		ModelOpts: []model.Option{
			model.CreateFieldOpt(func(*model.Field) *model.Field {
				return &model.Field{
					Name:     "Teacher",
					Type:     "Teacher",
					Relation: field.NewRelation("Teacher", "model.Teacher"),
				}
			}),
			model.ModifyFieldOpt(func(m *model.Field) *model.Field {
				if m.IsRelation() && m.Relation.Name() == "Teacher" {
					m.Relation = m.Relation.WithConditional("Instructor", int64(1))
				}
				return m
			}),
		},
	}).Preprocess()

	fields := getFields(nil, conf, nil)
	if len(fields) != 1 {
		t.Fatalf("fields length: got %d, want 1", len(fields))
	}
	conditional := fields[0].Relation.Conditional()
	if conditional == nil {
		t.Fatal("expected created relation to be marked conditional")
	}
	if conditional.Field != "Instructor" || conditional.Value != int64(1) {
		t.Fatalf("unexpected conditional metadata: %#v", conditional)
	}
}

func TestRelateConfigAppliesConditionalMetadata(t *testing.T) {
	relation := field.NewRelation("Teacher", "model.Teacher")
	config := &field.RelateConfig{
		Conditional: field.NewConditionalRelation("Instructor", int64(1)),
	}

	relation = config.ApplyConditional(relation)

	conditional := relation.Conditional()
	if conditional == nil {
		t.Fatal("expected relation to be marked conditional")
	}
	if conditional.Field != "Instructor" || conditional.Value != int64(1) {
		t.Fatalf("unexpected conditional metadata: %#v", conditional)
	}
}
