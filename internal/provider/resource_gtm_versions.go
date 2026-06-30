package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmContainerVersionResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmContainerVersionResource)(nil)

func NewGTMContainerVersionResource() resource.Resource {
	return &gtmContainerVersionResource{}
}

type gtmContainerVersionResource struct {
	client *marketingClient
}

type gtmContainerVersionModel struct {
	ID                 types.String `tfsdk:"id"`
	AccountID          types.String `tfsdk:"account_id"`
	ContainerID        types.String `tfsdk:"container_id"`
	WorkspaceID        types.String `tfsdk:"workspace_id"`
	Name               types.String `tfsdk:"name"`
	Notes              types.String `tfsdk:"notes"`
	Revision           types.String `tfsdk:"revision"`
	Path               types.String `tfsdk:"path"`
	ContainerVersionID types.String `tfsdk:"container_version_id"`
}

func (r *gtmContainerVersionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_container_version"
}

func (r *gtmContainerVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates an immutable GTM container version from a workspace.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"account_id":   replaceStringAttribute(),
			"container_id": replaceStringAttribute(),
			"workspace_id": replaceStringAttribute(),
			"name":         schema.StringAttribute{Required: true},
			"notes":        schema.StringAttribute{Optional: true},
			"revision": schema.StringAttribute{
				Optional:    true,
				Description: "Caller-controlled value that forces replacement when the workspace content changes.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"path":                 schema.StringAttribute{Computed: true},
			"container_version_id": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *gtmContainerVersionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *gtmContainerVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmContainerVersionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s:create_version", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.WorkspaceID.ValueString())
	body := map[string]any{
		"name":  plan.Name.ValueString(),
		"notes": plan.Notes.ValueString(),
	}
	var out struct {
		ContainerVersion map[string]any `json:"containerVersion"`
		CompilerError    bool           `json:"compilerError"`
	}
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), body, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GTM container version", err.Error())
		return
	}
	if out.CompilerError {
		resp.Diagnostics.AddError("GTM compiler error", "Google created no publishable version because the workspace has compiler errors.")
		return
	}

	resourcePath, _ := out.ContainerVersion["path"].(string)
	versionID, _ := out.ContainerVersion["containerVersionId"].(string)
	if resourcePath == "" {
		resp.Diagnostics.AddError("GTM response missing path", "Google did not return a container version path.")
		return
	}

	plan.ID = types.StringValue(resourcePath)
	plan.Path = types.StringValue(resourcePath)
	plan.ContainerVersionID = types.StringValue(versionID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmContainerVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmContainerVersionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gtmURL(state.Path.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GTM container version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmContainerVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("GTM versions are immutable", "Create a new googlemarketing_gtm_container_version when the workspace changes.")
}

func (r *gtmContainerVersionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmContainerVersionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(state.Path.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GTM container version", err.Error())
	}
}

var _ resource.Resource = (*gtmVersionPublicationResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmVersionPublicationResource)(nil)

func NewGTMVersionPublicationResource() resource.Resource {
	return &gtmVersionPublicationResource{}
}

type gtmVersionPublicationResource struct {
	client *marketingClient
}

type gtmVersionPublicationModel struct {
	ID          types.String `tfsdk:"id"`
	AccountID   types.String `tfsdk:"account_id"`
	ContainerID types.String `tfsdk:"container_id"`
	VersionID   types.String `tfsdk:"version_id"`
	Path        types.String `tfsdk:"path"`
}

func (r *gtmVersionPublicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_version_publication"
}

func (r *gtmVersionPublicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Publishes a GTM container version.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"account_id":   replaceStringAttribute(),
			"container_id": replaceStringAttribute(),
			"version_id":   replaceStringAttribute(),
			"path":         schema.StringAttribute{Computed: true},
		},
	}
}

func (r *gtmVersionPublicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *gtmVersionPublicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmVersionPublicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pathValue := fmt.Sprintf("accounts/%s/containers/%s/versions/%s", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.VersionID.ValueString())
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(pathValue+":publish"), nil, nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to publish GTM version", err.Error())
		return
	}

	plan.ID = types.StringValue(pathValue)
	plan.Path = types.StringValue(pathValue)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmVersionPublicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmVersionPublicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gtmURL(state.Path.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GTM published version", err.Error())
	}
}

func (r *gtmVersionPublicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("GTM publications are immutable", "Change version_id to publish another GTM version.")
}

func (r *gtmVersionPublicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func replaceStringAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}
