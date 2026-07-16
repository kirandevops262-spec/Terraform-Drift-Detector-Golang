package extract

import (
	"fmt"
	"strings"

	"github.com/terraform-drift-detector/golang/internal/state"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Options configures extraction behavior.
type Options struct {
	Provider string
	Region   string
}

// FromState converts Terraform state resources into normalized models.
func FromState(raw *state.RawState, opts Options) []models.Resource {
	var resources []models.Resource
	for _, res := range raw.Resources {
		if res.Mode != "managed" {
			continue
		}
		for i, inst := range res.Instances {
			if inst.Attributes == nil {
				continue
			}
			r := extractFromAttributes(res.Type, res.Name, res.Provider, inst.Attributes, opts)
			if len(res.Instances) > 1 {
				r.TerraformRef = fmt.Sprintf("%s.%s[%d]", res.Type, res.Name, i)
			}
			resources = append(resources, r)
		}
	}
	return resources
}

func extractFromAttributes(resType, name, provider string, attrs map[string]any, opts Options) models.Resource {
	providerName := normalizeProvider(provider, opts.Provider)
	region := opts.Region
	if r, ok := attrs["region"].(string); ok && r != "" {
		region = r
	}
	if r, ok := attrs["availability_zone"].(string); ok && region == "" && len(r) > 0 {
		region = r[:len(r)-1]
	}

	cloudID := stringAttr(attrs, "id")
	tags := extractTags(attrs)

	attributes := make(map[string]any)
	for k, v := range attrs {
		if k == "tags" || k == "tags_all" {
			continue
		}
		attributes[k] = v
	}

	ref := fmt.Sprintf("%s.%s", resType, name)
	id := fmt.Sprintf("%s:%s:%s", providerName, resType, cloudID)
	if cloudID == "" {
		id = fmt.Sprintf("%s:%s:%s", providerName, resType, ref)
	}

	return models.Resource{
		ID:           id,
		TerraformRef: ref,
		Type:         resType,
		Provider:     providerName,
		Region:       region,
		CloudID:      cloudID,
		Attributes:   attributes,
		Tags:         tags,
		Metadata: map[string]string{
			"source": "terraform_state",
		},
	}
}

// FromCloud converts raw cloud resources into normalized models.
func FromCloud(raw []RawCloudResource) []models.Resource {
	out := make([]models.Resource, 0, len(raw))
	for _, r := range raw {
		out = append(out, models.Resource{
			ID:           r.ID,
			TerraformRef: r.TerraformRef,
			Type:         r.Type,
			Provider:     r.Provider,
			Region:       r.Region,
			CloudID:      r.CloudID,
			Attributes:   r.Attributes,
			Tags:         r.Tags,
			Metadata:     r.Metadata,
		})
	}
	return out
}

// RawCloudResource is the intermediate cloud fetch result.
type RawCloudResource struct {
	ID           string
	TerraformRef string
	Type         string
	Provider     string
	Region       string
	CloudID      string
	Attributes   map[string]any
	Tags         map[string]string
	Metadata     map[string]string
}

func normalizeProvider(tfProvider, fallback string) string {
	if strings.Contains(tfProvider, "aws") {
		return "aws"
	}
	if strings.Contains(tfProvider, "azurerm") {
		return "azure"
	}
	if strings.Contains(tfProvider, "google") {
		return "gcp"
	}
	if fallback != "" {
		return fallback
	}
	return "unknown"
}

func extractTags(attrs map[string]any) map[string]string {
	tags := make(map[string]string)
	if t, ok := attrs["tags"].(map[string]any); ok {
		for k, v := range t {
			tags[k] = fmt.Sprint(v)
		}
	}
	if t, ok := attrs["tags_all"].(map[string]any); ok && len(tags) == 0 {
		for k, v := range t {
			tags[k] = fmt.Sprint(v)
		}
	}
	return tags
}

func stringAttr(attrs map[string]any, key string) string {
	if v, ok := attrs[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}
