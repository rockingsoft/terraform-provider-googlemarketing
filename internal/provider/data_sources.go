package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*gtmAccountsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*gtmAccountsDataSource)(nil)

func NewGTMAccountsDataSource() datasource.DataSource {
	return &gtmAccountsDataSource{}
}

func NewGTMWorkspacesDataSource() datasource.DataSource {
	return &gtmWorkspacesDataSource{}
}

type gtmAccountsDataSource struct {
	client *marketingClient
}

type gtmAccountsDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	AccountsJSON types.String `tfsdk:"accounts_json"`
}

type gtmWorkspacesDataSource struct {
	client *marketingClient
}

type gtmWorkspaceDataSourceItem struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Path        types.String `tfsdk:"path"`
}

type gtmWorkspacesDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	AccountID      types.String `tfsdk:"account_id"`
	ContainerID    types.String `tfsdk:"container_id"`
	WorkspacesJSON types.String `tfsdk:"workspaces_json"`
	Workspaces     types.List   `tfsdk:"workspaces"`
}

func (d *gtmAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_accounts"
}

func (d *gtmAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Google Tag Manager accounts visible to the configured credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"accounts_json": schema.StringAttribute{
				Computed:    true,
				Description: "Raw accounts array returned by the GTM API.",
			},
		},
	}
}

func (d *gtmAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *gtmAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var out struct {
		Account []map[string]any `json:"account"`
	}
	if err := d.client.doJSON(ctx, http.MethodGet, gtmURL("accounts"), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to list GTM accounts", err.Error())
		return
	}
	raw, err := encodeJSON(out.Account)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode GTM accounts", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &gtmAccountsDataSourceModel{
		ID:           types.StringValue("gtm_accounts"),
		AccountsJSON: types.StringValue(raw),
	})...)
}

var _ datasource.DataSource = (*gtmWorkspacesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*gtmWorkspacesDataSource)(nil)

func (d *gtmWorkspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_workspaces"
}

func (d *gtmWorkspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Google Tag Manager workspaces for a container.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"account_id":   schema.StringAttribute{Required: true},
			"container_id": schema.StringAttribute{Required: true},
			"workspaces_json": schema.StringAttribute{
				Computed:    true,
				Description: "Raw workspace array returned by the GTM API.",
			},
			"workspaces": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"workspace_id": schema.StringAttribute{Computed: true},
						"name":         schema.StringAttribute{Computed: true},
						"path":         schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *gtmWorkspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *gtmWorkspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config gtmWorkspacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out struct {
		Workspace []map[string]any `json:"workspace"`
	}
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces", config.AccountID.ValueString(), config.ContainerID.ValueString())
	if err := d.client.doJSON(ctx, http.MethodGet, gtmURL(apiPath), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to list GTM workspaces", err.Error())
		return
	}

	items := make([]gtmWorkspaceDataSourceItem, 0, len(out.Workspace))
	for _, workspace := range out.Workspace {
		workspaceID, _ := workspace["workspaceId"].(string)
		name, _ := workspace["name"].(string)
		pathValue, _ := workspace["path"].(string)
		items = append(items, gtmWorkspaceDataSourceItem{
			WorkspaceID: types.StringValue(workspaceID),
			Name:        types.StringValue(name),
			Path:        types.StringValue(pathValue),
		})
	}
	workspaces, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"workspace_id": types.StringType,
			"name":         types.StringType,
			"path":         types.StringType,
		},
	}, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := encodeJSON(out.Workspace)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode GTM workspaces", err.Error())
		return
	}

	state := gtmWorkspacesDataSourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s", config.AccountID.ValueString(), config.ContainerID.ValueString())),
		AccountID:      config.AccountID,
		ContainerID:    config.ContainerID,
		WorkspacesJSON: types.StringValue(raw),
		Workspaces:     workspaces,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

var _ datasource.DataSource = (*adsAccessibleCustomersDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*adsAccessibleCustomersDataSource)(nil)

func NewAdsAccessibleCustomersDataSource() datasource.DataSource {
	return &adsAccessibleCustomersDataSource{}
}

type adsAccessibleCustomersDataSource struct {
	client *marketingClient
}

type adsAccessibleCustomersDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	ResourceNames types.List   `tfsdk:"resource_names"`
}

func (d *adsAccessibleCustomersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ads_accessible_customers"
}

func (d *adsAccessibleCustomersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Google Ads customer resource names accessible to the configured credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"resource_names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Google Ads customer resource names.",
			},
		},
	}
}

func (d *adsAccessibleCustomersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *adsAccessibleCustomersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var out struct {
		ResourceNames []string `json:"resourceNames"`
	}
	url := fmt.Sprintf("%s/%s/customers:listAccessibleCustomers", adsBaseURL, d.client.adsAPIVersion)
	if err := d.client.doJSON(ctx, http.MethodGet, url, nil, &out, d.client.adsHeaders()); err != nil {
		resp.Diagnostics.AddError("Unable to list Google Ads accessible customers", err.Error())
		return
	}
	values, diags := types.ListValueFrom(ctx, types.StringType, out.ResourceNames)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &adsAccessibleCustomersDataSourceModel{
		ID:            types.StringValue("ads_accessible_customers"),
		ResourceNames: values,
	})...)
}
