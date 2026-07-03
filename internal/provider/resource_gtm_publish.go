package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmPublishResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmPublishResource)(nil)
var _ resource.ResourceWithModifyPlan = (*gtmPublishResource)(nil)

func NewGTMPublishResource() resource.Resource {
	return &gtmPublishResource{}
}

// gtmPublishResource is a long-lived, per-container "publisher". Unlike the
// old googlemarketing_gtm_container_release, changing version_name, notes,
// workspace_name, or publish never forces a replace: those are plain
// Optional/Required attributes with no plan modifiers, so changing them by
// itself only updates metadata for whenever the next real publish happens.
// A GTM version is only created when GTM itself reports the workspace has
// pending changes (checked in ModifyPlan for an accurate preview, and again
// authoritatively in Update/Create before writing).
type gtmPublishResource struct {
	client *marketingClient
}

type gtmPublishModel struct {
	ID                 types.String `tfsdk:"id"`
	AccountID          types.String `tfsdk:"account_id"`
	ContainerID        types.String `tfsdk:"container_id"`
	WorkspaceName      types.String `tfsdk:"workspace_name"`
	VersionName        types.String `tfsdk:"version_name"`
	Notes              types.String `tfsdk:"notes"`
	Publish            types.Bool   `tfsdk:"publish"`
	VersionPath        types.String `tfsdk:"version_path"`
	ContainerVersionID types.String `tfsdk:"container_version_id"`
	NewWorkspacePath   types.String `tfsdk:"new_workspace_path"`
	Published          types.Bool   `tfsdk:"published"`
}

func (r *gtmPublishResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_publish"
}

func (r *gtmPublishResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Publishes a Google Tag Manager container whenever its workspace has pending changes from googlemarketing_gtm_variable/gtm_trigger/gtm_tag resources, without requiring a manual depends_on list.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "accounts/{account_id}/containers/{container_id}.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_id":   entityReplaceStringAttribute("GTM account ID."),
			"container_id": entityReplaceStringAttribute("GTM container ID."),
			"workspace_name": schema.StringAttribute{
				Optional:    true,
				Description: "Workspace to publish from. Defaults to \"Default Workspace\". Changing it alone does not force a republish.",
			},
			"version_name": schema.StringAttribute{
				Required:    true,
				Description: "Name recorded on the GTM container version whenever a new version is actually created. Changing it alone does not force a republish.",
			},
			"notes": schema.StringAttribute{
				Optional:    true,
				Description: "Notes recorded on the GTM container version whenever a new version is actually created.",
			},
			"publish": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to publish the created container version, versus only creating it. Defaults to true.",
			},
			"version_path": schema.StringAttribute{
				Computed:      true,
				Description:   "Path of the most recently created GTM container version.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"container_version_id": schema.StringAttribute{
				Computed:      true,
				Description:   "ID of the most recently created GTM container version.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"new_workspace_path": schema.StringAttribute{
				Computed:      true,
				Description:   "Path of the workspace GTM created to replace the one consumed by the last publish.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"published": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the most recently created version was published.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *gtmPublishResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan previews whether the eventual apply will publish a new version,
// so `terraform plan` shows an update instead of silently doing nothing (or
// worse, showing no changes at all when entities changed). It checks two
// signals: the in-memory dirty registry that entity resources populate
// earlier in the same plan (works when this resource depends on the
// entities, directly or via a module boundary), and, as a fallback, GTM's
// own workspace status API (catches drift made outside Terraform, and any
// case where dependency ordering didn't put entity planning first).
func (r *gtmPublishResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil || req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var plan gtmPublishModel
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		return
	}
	var state gtmPublishModel
	if diags := req.State.Get(ctx, &state); diags.HasError() {
		return
	}

	accountID, containerID := plan.AccountID.ValueString(), plan.ContainerID.ValueString()
	dirty := state.VersionPath.IsNull() || state.VersionPath.ValueString() == "" || r.client.isGTMContainerDirty(accountID, containerID)
	if !dirty {
		if workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, plan.WorkspaceName.ValueString()); err == nil {
			if pending, statusErr := r.client.gtmWorkspaceHasPendingChanges(ctx, accountID, containerID, workspaceID); statusErr == nil {
				dirty = pending
			}
			// Lookup errors are swallowed here: ModifyPlan must not fail
			// the plan over a transient GTM API issue. Update/Create
			// re-check authoritatively and will surface any real problem.
		}
	}
	if !dirty {
		return
	}
	resp.Plan.SetAttribute(ctx, path.Root("version_path"), types.StringUnknown())
	resp.Plan.SetAttribute(ctx, path.Root("container_version_id"), types.StringUnknown())
	resp.Plan.SetAttribute(ctx, path.Root("new_workspace_path"), types.StringUnknown())
	resp.Plan.SetAttribute(ctx, path.Root("published"), types.BoolUnknown())
}

func (r *gtmPublishResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmPublishModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.createVersionAndPublish(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to publish GTM container", err.Error())
		return
	}
	plan.ID = types.StringValue(gtmContainerKey(plan.AccountID.ValueString(), plan.ContainerID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmPublishResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmPublishModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.VersionPath.IsNull() || state.VersionPath.ValueString() == "" {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gtmURL(state.VersionPath.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		// The version this resource last published is gone (rare: deleted
		// out of band). Clear the outputs so the next plan republishes
		// instead of erroring on a stale path.
		state.VersionPath = types.StringNull()
		state.ContainerVersionID = types.StringNull()
		state.Published = types.BoolValue(false)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GTM container version", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmPublishResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmPublishModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state gtmPublishModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID, containerID := plan.AccountID.ValueString(), plan.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, plan.WorkspaceName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to resolve GTM workspace", err.Error())
		return
	}
	pending := state.VersionPath.IsNull() || state.VersionPath.ValueString() == ""
	if !pending {
		pending, err = r.client.gtmWorkspaceHasPendingChanges(ctx, accountID, containerID, workspaceID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to check GTM workspace status", err.Error())
			return
		}
	}
	if !pending {
		// Only cosmetic fields (version_name, notes, workspace_name,
		// publish) changed. Nothing in the workspace needs publishing, so
		// keep the previous version outputs instead of creating a no-op
		// version.
		plan.VersionPath = state.VersionPath
		plan.ContainerVersionID = state.ContainerVersionID
		plan.NewWorkspacePath = state.NewWorkspacePath
		plan.Published = state.Published
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	if err := r.createVersionAndPublish(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to publish GTM container", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmPublishResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// GTM versions cannot be deleted through the API and stay published;
	// removing this resource only stops Terraform from managing further
	// publishes.
}

func (r *gtmPublishResource) createVersionAndPublish(ctx context.Context, plan *gtmPublishModel) error {
	accountID, containerID := plan.AccountID.ValueString(), plan.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, plan.WorkspaceName.ValueString())
	if err != nil {
		return err
	}

	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s:create_version", accountID, containerID, workspaceID)
	body := map[string]any{"name": plan.VersionName.ValueString()}
	if !plan.Notes.IsNull() && !plan.Notes.IsUnknown() && plan.Notes.ValueString() != "" {
		body["notes"] = plan.Notes.ValueString()
	}
	var out struct {
		ContainerVersion map[string]any `json:"containerVersion"`
		CompilerError    bool           `json:"compilerError"`
		NewWorkspacePath string         `json:"newWorkspacePath"`
	}
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), body, &out, nil); err != nil {
		return err
	}
	if out.CompilerError {
		return fmt.Errorf("Google created no publishable version because the workspace has compiler errors")
	}
	versionPath := stringFromMap(out.ContainerVersion, "path")
	if versionPath == "" {
		return fmt.Errorf("GTM response missing container version path")
	}
	versionID := stringFromMap(out.ContainerVersion, "containerVersionId")

	publish := true
	if !plan.Publish.IsNull() && !plan.Publish.IsUnknown() {
		publish = plan.Publish.ValueBool()
	}
	if publish {
		if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(versionPath+":publish"), nil, nil, nil); err != nil {
			return err
		}
	}

	plan.VersionPath = types.StringValue(versionPath)
	plan.ContainerVersionID = types.StringValue(versionID)
	plan.NewWorkspacePath = types.StringValue(out.NewWorkspacePath)
	plan.Published = types.BoolValue(publish)

	// create_version deletes the workspace it read from and GTM replaces it
	// with a new one (out.NewWorkspacePath); drop the cached workspace ID so
	// entity resources re-resolve against the workspace that now exists.
	r.client.invalidateGTMWorkspaces(accountID, containerID)
	r.client.clearGTMContainerDirty(accountID, containerID)
	return nil
}
