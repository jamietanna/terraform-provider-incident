package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"

	_ "embed"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/incident-io/terraform-provider-incident/internal/client"
	"github.com/motemen/go-loghttp"
)

var _ provider.Provider = &IncidentProvider{}

type IncidentProvider struct {
	version string
}

type IncidentProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &IncidentProvider{
			version: version,
		}
	}
}

func (p *IncidentProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "incident"
	resp.Version = p.version
}

func (p *IncidentProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This project is the official terraform provider for incident.io.",
		MarkdownDescription: `
This project is the official terraform provider for incident.io.

With this provider you manage configuration such as incident severities, roles,
custom fields and more inside of your incident.io account.

To view the full documentation of this provider, we recommend reading the
documentation on the [Terraform
Registry](https://registry.terraform.io/providers/incident-io/incident/latest).
`,
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "URL of the incident.io API",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for incident.io (https://app.incident.io/settings/api-keys). Sourced from the INCIDENT_API_KEY environment variable, if set.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *IncidentProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data IncidentProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var endpoint string
	if override := os.Getenv("INCIDENT_ENDPOINT"); override != "" {
		endpoint = override
	} else if data.Endpoint.IsNull() || data.Endpoint.IsUnknown() {
		endpoint = "https://api.incident.io"
	} else {
		endpoint = data.Endpoint.ValueString()
	}

	var apiKey string
	if data.APIKey.IsNull() || data.APIKey.IsUnknown() {
		apiKey = os.Getenv("INCIDENT_API_KEY")
	} else {
		apiKey = data.APIKey.ValueString()
	}

	bearerTokenProvider, bearerTokenProviderErr := securityprovider.NewSecurityProviderBearerToken(apiKey)
	if bearerTokenProviderErr != nil {
		panic(bearerTokenProviderErr)
	}

	base := cleanhttp.DefaultClient()
	base.Transport = &loghttp.Transport{
		Transport: cleanhttp.DefaultTransport(),
	}

	client, err := client.NewClientWithResponses(
		endpoint,
		client.WithHTTPClient(base),
		client.WithRequestEditorFn(bearerTokenProvider.Intercept),
		// Add a user-agent so we can tell which version these requests came from.
		client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("user-agent", fmt.Sprintf("terraform-provider-incident/%s", p.version))
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *IncidentProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewIncidentCatalogEntryResource,
		NewIncidentCatalogTypeAttributesResource,
		NewIncidentCatalogTypeResource,
		NewIncidentCustomFieldOptionResource,
		NewIncidentCustomFieldResource,
		NewIncidentRoleResource,
		NewIncidentSeverityResource,
		NewIncidentStatusResource,
	}
}

func (p *IncidentProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
