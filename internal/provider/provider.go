package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*marketingProvider)(nil)

func New() provider.Provider {
	return &marketingProvider{}
}

type marketingProvider struct{}

type providerModel struct {
	CredentialsFile    types.String `tfsdk:"credentials_file"`
	CredentialsJSON    types.String `tfsdk:"credentials_json"`
	AdsDeveloperToken  types.String `tfsdk:"ads_developer_token"`
	AdsLoginCustomerID types.String `tfsdk:"ads_login_customer_id"`
	AdsAPIVersion      types.String `tfsdk:"ads_api_version"`
	GTMRequestInterval types.Int64  `tfsdk:"gtm_request_interval_ms"`
}

func (p *marketingProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "googlemarketing"
}

func (p *marketingProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Google Tag Manager, Google Analytics 4 Admin, and Google Ads management APIs.",
		Attributes: map[string]schema.Attribute{
			"credentials_file": schema.StringAttribute{
				Optional:    true,
				Description: "Path to a Google service account or OAuth client credentials JSON file. Defaults to GOOGLE_APPLICATION_CREDENTIALS or ADC.",
			},
			"credentials_json": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Inline Google credentials JSON. Defaults to GOOGLEMARKETING_CREDENTIALS_JSON.",
			},
			"ads_developer_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Google Ads developer token. Defaults to GOOGLE_ADS_DEVELOPER_TOKEN.",
			},
			"ads_login_customer_id": schema.StringAttribute{
				Optional:    true,
				Description: "Optional Google Ads manager account login customer ID. Defaults to GOOGLE_ADS_LOGIN_CUSTOMER_ID.",
			},
			"ads_api_version": schema.StringAttribute{
				Optional:    true,
				Description: "Google Ads API version path segment, for example v24. Defaults to GOOGLE_ADS_API_VERSION or v24.",
			},
			"gtm_request_interval_ms": schema.Int64Attribute{
				Optional:    true,
				Description: "Minimum delay in milliseconds between Google Tag Manager API requests. Defaults to GOOGLEMARKETING_GTM_REQUEST_INTERVAL_MS or 4000 to respect GTM's 25 requests per 100 seconds project quota. Set to 0 to disable provider-side pacing.",
			},
		},
	}
}

func (p *marketingProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var model providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gtmInterval, err := gtmRequestIntervalFrom(model.GTMRequestInterval, os.Getenv("GOOGLEMARKETING_GTM_REQUEST_INTERVAL_MS"))
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("gtm_request_interval_ms"), "Invalid GTM request interval", err.Error())
		return
	}

	cfg := clientConfig{
		CredentialsFile:    stringFrom(model.CredentialsFile, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")),
		CredentialsJSON:    stringFrom(model.CredentialsJSON, os.Getenv("GOOGLEMARKETING_CREDENTIALS_JSON")),
		AdsDeveloperToken:  stringFrom(model.AdsDeveloperToken, os.Getenv("GOOGLE_ADS_DEVELOPER_TOKEN")),
		AdsLoginCustomerID: stringFrom(model.AdsLoginCustomerID, os.Getenv("GOOGLE_ADS_LOGIN_CUSTOMER_ID")),
		AdsAPIVersion:      stringFrom(model.AdsAPIVersion, os.Getenv("GOOGLE_ADS_API_VERSION")),
		GTMRequestInterval: gtmInterval,
	}
	if cfg.AdsAPIVersion == "" {
		cfg.AdsAPIVersion = "v24"
	}

	client, err := newClient(ctx, cfg)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("credentials_file"), "Unable to configure Google client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *marketingProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGTMContainerReleaseResource,
		NewGA4AdminResource,
		NewGA4PropertyResource,
		NewGA4WebDataStreamResource,
		NewGA4KeyEventResource,
		NewGA4CustomDimensionResource,
		NewGA4CustomMetricResource,
		NewGA4DataRetentionSettingsResource,
		NewAdsMutateResource,
		NewAdsConversionActionResource,
	}
}

func (p *marketingProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGTMAccountsDataSource,
		NewAdsAccessibleCustomersDataSource,
	}
}

func stringFrom(v types.String, fallback string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	return fallback
}

func gtmRequestIntervalFrom(v types.Int64, fallback string) (time.Duration, error) {
	if !v.IsNull() && !v.IsUnknown() {
		if v.ValueInt64() < 0 {
			return 0, fmt.Errorf("must be greater than or equal to 0")
		}
		return time.Duration(v.ValueInt64()) * time.Millisecond, nil
	}
	if fallback != "" {
		return parseDurationMillis(fallback)
	}
	return defaultGTMRequestInterval, nil
}
