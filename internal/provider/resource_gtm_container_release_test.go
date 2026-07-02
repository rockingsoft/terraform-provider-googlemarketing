package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func TestGTMContainerReleaseSchemaRequiresReplacementForReleaseInputs(t *testing.T) {
	ctx := context.Background()
	resourceSchema := gtmContainerReleaseSchema(t, ctx)

	for _, name := range []string{
		"account_id",
		"container_id",
		"workspace_name",
		"name",
		"notes",
		"revision",
	} {
		t.Run("attribute_"+name, func(t *testing.T) {
			attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
			if !ok {
				t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
			}
			assertStringRequiresReplace(t, name, attribute)
		})
	}

	publish, ok := resourceSchema.Attributes["publish"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("publish is %T, want schema.BoolAttribute", resourceSchema.Attributes["publish"])
	}
	assertBoolRequiresReplace(t, "publish", publish)

	for _, name := range []string{"variable", "trigger", "tag", "ga4_event_tag"} {
		t.Run("block_"+name, func(t *testing.T) {
			block, ok := resourceSchema.Blocks[name].(schema.ListNestedBlock)
			if !ok {
				t.Fatalf("%s is %T, want schema.ListNestedBlock", name, resourceSchema.Blocks[name])
			}
			assertListRequiresReplace(t, name, block)
		})
	}
}

func TestGTMContainerReleaseComputedVersionOutputsDoNotUsePriorState(t *testing.T) {
	ctx := context.Background()
	resourceSchema := gtmContainerReleaseSchema(t, ctx)

	for _, name := range []string{"id", "workspace_id_used", "container_version_id", "version_path"} {
		t.Run(name, func(t *testing.T) {
			attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
			if !ok {
				t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
			}
			if got := len(attribute.PlanModifiers); got != 0 {
				t.Fatalf("%s has %d plan modifiers, want 0 so newly created GTM versions can be returned", name, got)
			}
		})
	}
}

func gtmContainerReleaseSchema(t *testing.T, ctx context.Context) schema.Schema {
	t.Helper()

	var resp resource.SchemaResponse
	NewGTMContainerReleaseResource().Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %s", resp.Diagnostics)
	}
	return resp.Schema
}

func assertStringRequiresReplace(t *testing.T, name string, attribute schema.StringAttribute) {
	t.Helper()
	if len(attribute.PlanModifiers) == 0 {
		t.Fatalf("%s has no string plan modifiers", name)
	}
	want := reflect.TypeOf(stringplanmodifier.RequiresReplace())
	for _, modifier := range attribute.PlanModifiers {
		if reflect.TypeOf(modifier) == want {
			return
		}
	}
	t.Fatalf("%s does not require replacement", name)
}

func assertBoolRequiresReplace(t *testing.T, name string, attribute schema.BoolAttribute) {
	t.Helper()
	if len(attribute.PlanModifiers) == 0 {
		t.Fatalf("%s has no bool plan modifiers", name)
	}
	want := reflect.TypeOf(boolplanmodifier.RequiresReplace())
	for _, modifier := range attribute.PlanModifiers {
		if reflect.TypeOf(modifier) == want {
			return
		}
	}
	t.Fatalf("%s does not require replacement", name)
}

func assertListRequiresReplace(t *testing.T, name string, block schema.ListNestedBlock) {
	t.Helper()
	if len(block.PlanModifiers) == 0 {
		t.Fatalf("%s has no list plan modifiers", name)
	}
	want := reflect.TypeOf(listplanmodifier.RequiresReplace())
	for _, modifier := range block.PlanModifiers {
		if reflect.TypeOf(modifier) == want {
			return
		}
	}
	t.Fatalf("%s does not require replacement", name)
}
