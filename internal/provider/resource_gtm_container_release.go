package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmContainerReleaseResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmContainerReleaseResource)(nil)

func NewGTMContainerReleaseResource() resource.Resource {
	return &gtmContainerReleaseResource{}
}

type gtmContainerReleaseResource struct {
	client *marketingClient
}

type gtmContainerReleaseModel struct {
	ID                 types.String         `tfsdk:"id"`
	AccountID          types.String         `tfsdk:"account_id"`
	ContainerID        types.String         `tfsdk:"container_id"`
	WorkspaceName      types.String         `tfsdk:"workspace_name"`
	Name               types.String         `tfsdk:"name"`
	Notes              types.String         `tfsdk:"notes"`
	Revision           types.String         `tfsdk:"revision"`
	Publish            types.Bool           `tfsdk:"publish"`
	WorkspaceIDUsed    types.String         `tfsdk:"workspace_id_used"`
	ContainerVersionID types.String         `tfsdk:"container_version_id"`
	VersionPath        types.String         `tfsdk:"version_path"`
	Published          types.Bool           `tfsdk:"published"`
	Variables          []gtmReleaseVariable `tfsdk:"variable"`
	Triggers           []gtmReleaseTrigger  `tfsdk:"trigger"`
	Tags               []gtmReleaseTag      `tfsdk:"tag"`
	GA4EventTags       []gtmReleaseGA4Tag   `tfsdk:"ga4_event_tag"`
}

type gtmReleaseVariable struct {
	Key           types.String `tfsdk:"key"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	Notes         types.String `tfsdk:"notes"`
	Value         types.String `tfsdk:"value"`
	DataLayerName types.String `tfsdk:"data_layer_name"`
	CookieName    types.String `tfsdk:"cookie_name"`
	JavaScript    types.String `tfsdk:"javascript"`
}

type gtmReleaseTrigger struct {
	Key             types.String `tfsdk:"key"`
	Name            types.String `tfsdk:"name"`
	Type            types.String `tfsdk:"type"`
	Notes           types.String `tfsdk:"notes"`
	CustomEventName types.String `tfsdk:"custom_event_name"`
	FilterVariable  types.String `tfsdk:"filter_variable"`
	FilterOperator  types.String `tfsdk:"filter_operator"`
	FilterValue     types.String `tfsdk:"filter_value"`
}

type gtmReleaseTag struct {
	Key                 types.String `tfsdk:"key"`
	Name                types.String `tfsdk:"name"`
	Type                types.String `tfsdk:"type"`
	Notes               types.String `tfsdk:"notes"`
	HTML                types.String `tfsdk:"html"`
	MeasurementID       types.String `tfsdk:"measurement_id"`
	EventName           types.String `tfsdk:"event_name"`
	ConversionID        types.String `tfsdk:"conversion_id"`
	ConversionLabel     types.String `tfsdk:"conversion_label"`
	TriggerKeys         types.List   `tfsdk:"trigger_keys"`
	BlockingTriggerKeys types.List   `tfsdk:"blocking_trigger_keys"`
}

type gtmReleaseGA4Tag struct {
	Key                   types.String `tfsdk:"key"`
	Name                  types.String `tfsdk:"name"`
	Notes                 types.String `tfsdk:"notes"`
	EventName             types.String `tfsdk:"event_name"`
	MeasurementIDOverride types.String `tfsdk:"measurement_id_override"`
	TriggerKeys           types.List   `tfsdk:"trigger_keys"`
	BlockingTriggerKeys   types.List   `tfsdk:"blocking_trigger_keys"`
}

func (r *gtmContainerReleaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_container_release"
}

func (r *gtmContainerReleaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Publishes a complete Google Tag Manager container release from logical variables, triggers, and tags.",
		Attributes: map[string]schema.Attribute{
			"id":                   releaseComputedString("Published GTM version path."),
			"account_id":           schema.StringAttribute{Required: true},
			"container_id":         schema.StringAttribute{Required: true},
			"workspace_name":       schema.StringAttribute{Optional: true, Description: "Workspace name to use as the editable release workspace. Defaults to Default Workspace."},
			"name":                 schema.StringAttribute{Required: true},
			"notes":                schema.StringAttribute{Optional: true},
			"revision":             schema.StringAttribute{Required: true, Description: "Caller-controlled release fingerprint. A changed revision publishes a new GTM version."},
			"publish":              schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether to publish the created container version. Defaults to true."},
			"workspace_id_used":    releaseComputedString("Workspace ID used for the most recent release operation."),
			"container_version_id": releaseComputedString("Published or created container version ID."),
			"version_path":         releaseComputedString("GTM container version path."),
			"published":            schema.BoolAttribute{Computed: true},
		},
		Blocks: map[string]schema.Block{
			"variable":      schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: gtmReleaseVariableAttributes()}},
			"trigger":       schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: gtmReleaseTriggerAttributes()}},
			"tag":           schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: gtmReleaseTagAttributes()}},
			"ga4_event_tag": schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: gtmReleaseGA4TagAttributes()}},
		},
	}
}

func releaseComputedString(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Computed:    true,
		Description: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

func gtmReleaseVariableAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key":             schema.StringAttribute{Required: true},
		"name":            schema.StringAttribute{Required: true},
		"type":            schema.StringAttribute{Required: true},
		"notes":           schema.StringAttribute{Optional: true},
		"value":           schema.StringAttribute{Optional: true, Sensitive: true},
		"data_layer_name": schema.StringAttribute{Optional: true},
		"cookie_name":     schema.StringAttribute{Optional: true, Sensitive: true},
		"javascript":      schema.StringAttribute{Optional: true, Sensitive: true},
	}
}

func gtmReleaseTriggerAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key":               schema.StringAttribute{Required: true},
		"name":              schema.StringAttribute{Required: true},
		"type":              schema.StringAttribute{Required: true},
		"notes":             schema.StringAttribute{Optional: true},
		"custom_event_name": schema.StringAttribute{Optional: true},
		"filter_variable":   schema.StringAttribute{Optional: true},
		"filter_operator":   schema.StringAttribute{Optional: true},
		"filter_value":      schema.StringAttribute{Optional: true},
	}
}

func gtmReleaseTagAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key":                   schema.StringAttribute{Required: true},
		"name":                  schema.StringAttribute{Required: true},
		"type":                  schema.StringAttribute{Required: true},
		"notes":                 schema.StringAttribute{Optional: true},
		"html":                  schema.StringAttribute{Optional: true, Sensitive: true},
		"measurement_id":        schema.StringAttribute{Optional: true},
		"event_name":            schema.StringAttribute{Optional: true},
		"conversion_id":         schema.StringAttribute{Optional: true},
		"conversion_label":      schema.StringAttribute{Optional: true, Sensitive: true},
		"trigger_keys":          schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"blocking_trigger_keys": schema.ListAttribute{Optional: true, ElementType: types.StringType},
	}
}

func gtmReleaseGA4TagAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key":                     schema.StringAttribute{Required: true},
		"name":                    schema.StringAttribute{Required: true},
		"notes":                   schema.StringAttribute{Optional: true},
		"event_name":              schema.StringAttribute{Required: true},
		"measurement_id_override": schema.StringAttribute{Required: true},
		"trigger_keys":            schema.ListAttribute{Required: true, ElementType: types.StringType},
		"blocking_trigger_keys":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
	}
}

func (r *gtmContainerReleaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gtmContainerReleaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmContainerReleaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRelease(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to publish GTM container release", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmContainerReleaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmContainerReleaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.VersionPath.IsNull() || state.VersionPath.IsUnknown() || state.VersionPath.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gtmURL(state.VersionPath.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GTM container release", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmContainerReleaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmContainerReleaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRelease(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to publish GTM container release", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmContainerReleaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *gtmContainerReleaseResource) applyRelease(ctx context.Context, plan *gtmContainerReleaseModel) error {
	workspaceID, err := r.currentWorkspaceID(ctx, plan)
	if err != nil {
		return err
	}
	plan.WorkspaceIDUsed = types.StringValue(workspaceID)

	if err := validateGTMRelease(plan); err != nil {
		return err
	}

	triggerIDs, err := r.upsertReleaseTriggers(ctx, plan, workspaceID)
	if err != nil {
		return err
	}
	if err := r.upsertReleaseVariables(ctx, plan, workspaceID); err != nil {
		return err
	}
	if err := r.upsertReleaseTags(ctx, plan, workspaceID, triggerIDs); err != nil {
		return err
	}

	versionPath, versionID, err := r.createReleaseVersion(ctx, plan, workspaceID)
	if err != nil {
		return err
	}
	plan.ID = types.StringValue(versionPath)
	plan.VersionPath = types.StringValue(versionPath)
	plan.ContainerVersionID = types.StringValue(versionID)

	publish := true
	if !plan.Publish.IsNull() && !plan.Publish.IsUnknown() {
		publish = plan.Publish.ValueBool()
	}
	if publish {
		if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(versionPath+":publish"), nil, nil, nil); err != nil {
			return err
		}
	}
	plan.Published = types.BoolValue(publish)
	return nil
}

func (r *gtmContainerReleaseResource) currentWorkspaceID(ctx context.Context, plan *gtmContainerReleaseModel) (string, error) {
	workspaceName := "Default Workspace"
	if !plan.WorkspaceName.IsNull() && !plan.WorkspaceName.IsUnknown() && plan.WorkspaceName.ValueString() != "" {
		workspaceName = plan.WorkspaceName.ValueString()
	}
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces", plan.AccountID.ValueString(), plan.ContainerID.ValueString())
	var out struct {
		Workspace []map[string]any `json:"workspace"`
	}
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(apiPath), nil, &out, nil); err != nil {
		return "", err
	}
	for _, workspace := range out.Workspace {
		if stringFromMap(workspace, "name") == workspaceName {
			workspaceID := stringFromMap(workspace, "workspaceId")
			if workspaceID == "" {
				return "", fmt.Errorf("workspace %q did not include workspaceId", workspaceName)
			}
			return workspaceID, nil
		}
	}
	return "", fmt.Errorf("workspace %q not found in container %s", workspaceName, plan.ContainerID.ValueString())
}

func validateGTMRelease(plan *gtmContainerReleaseModel) error {
	seen := map[string]string{}
	for _, variable := range plan.Variables {
		if err := validateReleaseKey(seen, "variable", variable.Key.ValueString()); err != nil {
			return err
		}
	}
	for _, trigger := range plan.Triggers {
		if err := validateReleaseKey(seen, "trigger", trigger.Key.ValueString()); err != nil {
			return err
		}
	}
	for _, tag := range plan.Tags {
		if err := validateReleaseKey(seen, "tag", tag.Key.ValueString()); err != nil {
			return err
		}
	}
	for _, tag := range plan.GA4EventTags {
		if err := validateReleaseKey(seen, "ga4_event_tag", tag.Key.ValueString()); err != nil {
			return err
		}
	}
	return nil
}

func validateReleaseKey(seen map[string]string, kind, key string) error {
	if key == "" {
		return fmt.Errorf("%s key must not be empty", kind)
	}
	if prior, ok := seen[key]; ok {
		return fmt.Errorf("%s key %q duplicates %s key", kind, key, prior)
	}
	seen[key] = kind
	return nil
}

func (r *gtmContainerReleaseResource) upsertReleaseVariables(ctx context.Context, plan *gtmContainerReleaseModel, workspaceID string) error {
	collection := gtmWorkspaceCollectionPath(plan.AccountID.ValueString(), plan.ContainerID.ValueString(), workspaceID, "variables")
	for _, variable := range plan.Variables {
		model := gtmTypedWorkspaceEntityModel{
			Name:          variable.Name,
			Type:          variable.Type,
			Notes:         variable.Notes,
			Value:         variable.Value,
			DataLayerName: variable.DataLayerName,
			CookieName:    variable.CookieName,
			JavaScript:    variable.JavaScript,
		}
		if _, err := r.upsertReleaseEntity(ctx, collection, variable.Name.ValueString(), buildGTMPayload(ctx, "variable", model)); err != nil {
			return fmt.Errorf("variable %q: %w", variable.Key.ValueString(), err)
		}
	}
	return nil
}

func (r *gtmContainerReleaseResource) upsertReleaseTriggers(ctx context.Context, plan *gtmContainerReleaseModel, workspaceID string) (map[string]string, error) {
	collection := gtmWorkspaceCollectionPath(plan.AccountID.ValueString(), plan.ContainerID.ValueString(), workspaceID, "triggers")
	triggerIDs := map[string]string{}
	for _, trigger := range plan.Triggers {
		model := gtmTypedWorkspaceEntityModel{
			Name:            trigger.Name,
			Type:            trigger.Type,
			Notes:           trigger.Notes,
			CustomEventName: trigger.CustomEventName,
			FilterVariable:  trigger.FilterVariable,
			FilterOperator:  trigger.FilterOperator,
			FilterValue:     trigger.FilterValue,
		}
		out, err := r.upsertReleaseEntity(ctx, collection, trigger.Name.ValueString(), buildGTMPayload(ctx, "trigger", model))
		if err != nil {
			return nil, fmt.Errorf("trigger %q: %w", trigger.Key.ValueString(), err)
		}
		id := stringFromMap(out, "triggerId")
		if id == "" {
			return nil, fmt.Errorf("trigger %q response missing triggerId", trigger.Key.ValueString())
		}
		triggerIDs[trigger.Key.ValueString()] = id
	}
	return triggerIDs, nil
}

func (r *gtmContainerReleaseResource) upsertReleaseTags(ctx context.Context, plan *gtmContainerReleaseModel, workspaceID string, triggerIDs map[string]string) error {
	collection := gtmWorkspaceCollectionPath(plan.AccountID.ValueString(), plan.ContainerID.ValueString(), workspaceID, "tags")
	for _, tag := range plan.Tags {
		firingIDs, err := releaseTriggerIDs(ctx, tag.TriggerKeys, triggerIDs)
		if err != nil {
			return fmt.Errorf("tag %q: %w", tag.Key.ValueString(), err)
		}
		blockingIDs, err := releaseTriggerIDs(ctx, tag.BlockingTriggerKeys, triggerIDs)
		if err != nil {
			return fmt.Errorf("tag %q: %w", tag.Key.ValueString(), err)
		}
		model := gtmTypedWorkspaceEntityModel{
			Name:               tag.Name,
			Type:               tag.Type,
			Notes:              tag.Notes,
			HTML:               tag.HTML,
			MeasurementID:      tag.MeasurementID,
			EventName:          tag.EventName,
			ConversionID:       tag.ConversionID,
			ConversionLabel:    tag.ConversionLabel,
			FiringTriggerIDs:   stringListValueAllowEmpty(firingIDs),
			BlockingTriggerIDs: stringListValue(blockingIDs),
		}
		if _, err := r.upsertReleaseEntity(ctx, collection, tag.Name.ValueString(), buildGTMPayload(ctx, "tag", model)); err != nil {
			return fmt.Errorf("tag %q: %w", tag.Key.ValueString(), err)
		}
	}
	for _, tag := range plan.GA4EventTags {
		firingIDs, err := releaseTriggerIDs(ctx, tag.TriggerKeys, triggerIDs)
		if err != nil {
			return fmt.Errorf("ga4_event_tag %q: %w", tag.Key.ValueString(), err)
		}
		blockingIDs, err := releaseTriggerIDs(ctx, tag.BlockingTriggerKeys, triggerIDs)
		if err != nil {
			return fmt.Errorf("ga4_event_tag %q: %w", tag.Key.ValueString(), err)
		}
		model := gtmGA4EventTagModel{
			Name:                  tag.Name,
			Notes:                 tag.Notes,
			EventName:             tag.EventName,
			MeasurementIDOverride: tag.MeasurementIDOverride,
			TriggerIDs:            stringListValueAllowEmpty(firingIDs),
			BlockingTriggerIDs:    stringListValue(blockingIDs),
		}
		if err := validateGTMGA4EventTag(model); err != nil {
			return fmt.Errorf("ga4_event_tag %q: %w", tag.Key.ValueString(), err)
		}
		if _, err := r.upsertReleaseEntity(ctx, collection, tag.Name.ValueString(), buildGTMGA4EventTagPayload(ctx, model)); err != nil {
			return fmt.Errorf("ga4_event_tag %q: %w", tag.Key.ValueString(), err)
		}
	}
	return nil
}

func releaseTriggerIDs(ctx context.Context, keys types.List, triggerIDs map[string]string) ([]string, error) {
	keyValues := stringList(ctx, keys)
	ids := make([]string, 0, len(keyValues))
	for _, key := range keyValues {
		id, ok := triggerIDs[key]
		if !ok {
			return nil, fmt.Errorf("unknown trigger key %q", key)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *gtmContainerReleaseResource) upsertReleaseEntity(ctx context.Context, collectionPath, name string, payload map[string]any) (map[string]any, error) {
	existing, err := r.client.getGTMCollection(ctx, collectionPath)
	if err != nil {
		return nil, err
	}
	pathValue := ""
	for path, item := range existing {
		if stringFromMap(item, "name") == name {
			pathValue = path
			break
		}
	}
	method := http.MethodPost
	apiPath := collectionPath
	if pathValue != "" {
		method = http.MethodPut
		apiPath = pathValue
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, method, gtmURL(apiPath), payload, &out, nil); err != nil {
		return nil, err
	}
	r.client.invalidateGTMCollection(collectionPath)
	if len(out) == 0 && pathValue != "" {
		refreshed, err := r.client.getGTMCollection(ctx, collectionPath)
		if err != nil {
			return nil, err
		}
		out = refreshed[pathValue]
	}
	return out, nil
}

func (r *gtmContainerReleaseResource) createReleaseVersion(ctx context.Context, plan *gtmContainerReleaseModel, workspaceID string) (string, string, error) {
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s:create_version", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), workspaceID)
	body := map[string]any{
		"name":  plan.Name.ValueString(),
		"notes": plan.Notes.ValueString(),
	}
	var out struct {
		ContainerVersion map[string]any `json:"containerVersion"`
		CompilerError    bool           `json:"compilerError"`
		SyncStatus       map[string]any `json:"syncStatus"`
	}
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), body, &out, nil); err != nil {
		return "", "", err
	}
	if out.CompilerError {
		return "", "", fmt.Errorf("Google created no publishable version because the workspace has compiler errors")
	}
	versionPath := stringFromMap(out.ContainerVersion, "path")
	versionID := stringFromMap(out.ContainerVersion, "containerVersionId")
	if versionPath == "" {
		return "", "", fmt.Errorf("GTM response missing container version path")
	}
	return versionPath, versionID, nil
}
