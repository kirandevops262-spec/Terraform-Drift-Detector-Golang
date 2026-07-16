package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/terraform-drift-detector/golang/internal/cloud"
	"github.com/terraform-drift-detector/golang/internal/extract"
)

// DefaultResourceTypes are fetched when none are specified.
var DefaultResourceTypes = []string{
	"aws_instance",
	"aws_s3_bucket",
	"aws_vpc",
	"aws_subnet",
	"aws_security_group",
	"aws_lambda_function",
}

// Provider implements cloud.Provider for AWS.
type Provider struct {
	loadConfig func(ctx context.Context, region string) (aws.Config, error)
}

// NewProvider creates an AWS cloud provider using default credential chain.
func NewProvider() *Provider {
	return &Provider{
		loadConfig: func(ctx context.Context, region string) (aws.Config, error) {
			return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		},
	}
}

func (p *Provider) Name() string { return "aws" }

func (p *Provider) SupportedTypes() []string {
	return append([]string(nil), DefaultResourceTypes...)
}

func (p *Provider) Fetch(ctx context.Context, scope cloud.FetchScope) ([]extract.RawCloudResource, error) {
	types := scope.ResourceTypes
	if len(types) == 0 {
		types = DefaultResourceTypes
	}
	regions := scope.Regions
	if len(regions) == 0 {
		regions = []string{"us-east-1"}
	}

	var (
		mu    sync.Mutex
		all   []extract.RawCloudResource
		errs  []error
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 4)
	)

	for _, region := range regions {
		region := region
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg, err := p.loadConfig(ctx, region)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("aws config %s: %w", region, err))
				mu.Unlock()
				return
			}
			for _, t := range types {
				t := t
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					res, err := p.fetchType(ctx, cfg, region, t)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						errs = append(errs, fmt.Errorf("%s in %s: %w", t, region, err))
						return
					}
					all = append(all, res...)
				}()
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 && len(all) == 0 {
		return nil, fmt.Errorf("aws fetch failed: %v", errs[0])
	}
	return all, nil
}

func (p *Provider) fetchType(ctx context.Context, cfg aws.Config, region, resType string) ([]extract.RawCloudResource, error) {
	switch resType {
	case "aws_instance":
		return p.fetchInstances(ctx, cfg, region)
	case "aws_s3_bucket":
		return p.fetchS3Buckets(ctx, cfg, region)
	case "aws_vpc":
		return p.fetchVPCs(ctx, cfg, region)
	case "aws_subnet":
		return p.fetchSubnets(ctx, cfg, region)
	case "aws_security_group":
		return p.fetchSecurityGroups(ctx, cfg, region)
	case "aws_lambda_function":
		return p.fetchLambdaFunctions(ctx, cfg, region)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resType)
	}
}

func (p *Provider) fetchInstances(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			if inst.InstanceId == nil {
				continue
			}
			tags := tagsToMap(inst.Tags)
			attrs := map[string]any{
				"instance_type":   string(inst.InstanceType),
				"ami":             aws.ToString(inst.ImageId),
				"subnet_id":       aws.ToString(inst.SubnetId),
				"vpc_security_group_ids": sgIDs(inst.SecurityGroups),
				"monitoring":      inst.Monitoring != nil && inst.Monitoring.State == ec2types.MonitoringStateEnabled,
				"ebs_optimized":   aws.ToBool(inst.EbsOptimized),
			}
			if inst.State != nil {
				attrs["instance_state"] = string(inst.State.Name)
			}
			resources = append(resources, extract.RawCloudResource{
				ID:         fmt.Sprintf("aws:aws_instance:%s", *inst.InstanceId),
				Type:       "aws_instance",
				Provider:   "aws",
				Region:     region,
				CloudID:    *inst.InstanceId,
				Attributes: attrs,
				Tags:       tags,
				Metadata:   map[string]string{"source": "aws_api"},
			})
		}
	}
	return resources, nil
}

func (p *Provider) fetchS3Buckets(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := s3.NewFromConfig(cfg)
	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, b := range out.Buckets {
		name := aws.ToString(b.Name)
		locOut, _ := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: b.Name})
		bucketRegion := region
		if locOut != nil && locOut.LocationConstraint != "" {
			bucketRegion = string(locOut.LocationConstraint)
		} else if locOut != nil && locOut.LocationConstraint == "" {
			bucketRegion = "us-east-1"
		}
		tags := map[string]string{}
		tagsOut, err := client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: b.Name})
		if err == nil {
			for _, t := range tagsOut.TagSet {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
		}
		resources = append(resources, extract.RawCloudResource{
			ID:       fmt.Sprintf("aws:aws_s3_bucket:%s", name),
			Type:     "aws_s3_bucket",
			Provider: "aws",
			Region:   bucketRegion,
			CloudID:  name,
			Attributes: map[string]any{
				"bucket": name,
			},
			Tags:     tags,
			Metadata: map[string]string{"source": "aws_api"},
		})
	}
	return resources, nil
}

func (p *Provider) fetchVPCs(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, vpc := range out.Vpcs {
		if vpc.VpcId == nil {
			continue
		}
		resources = append(resources, extract.RawCloudResource{
			ID:       fmt.Sprintf("aws:aws_vpc:%s", *vpc.VpcId),
			Type:     "aws_vpc",
			Provider: "aws",
			Region:   region,
			CloudID:  *vpc.VpcId,
			Attributes: map[string]any{
				"cidr_block": aws.ToString(vpc.CidrBlock),
			},
			Tags:     tagsToMap(vpc.Tags),
			Metadata: map[string]string{"source": "aws_api"},
		})
	}
	return resources, nil
}

func (p *Provider) fetchSubnets(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, sn := range out.Subnets {
		if sn.SubnetId == nil {
			continue
		}
		resources = append(resources, extract.RawCloudResource{
			ID:       fmt.Sprintf("aws:aws_subnet:%s", *sn.SubnetId),
			Type:     "aws_subnet",
			Provider: "aws",
			Region:   region,
			CloudID:  *sn.SubnetId,
			Attributes: map[string]any{
				"cidr_block":        aws.ToString(sn.CidrBlock),
				"vpc_id":            aws.ToString(sn.VpcId),
				"availability_zone": aws.ToString(sn.AvailabilityZone),
				"map_public_ip_on_launch": aws.ToBool(sn.MapPublicIpOnLaunch),
			},
			Tags:     tagsToMap(sn.Tags),
			Metadata: map[string]string{"source": "aws_api"},
		})
	}
	return resources, nil
}

func (p *Provider) fetchSecurityGroups(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, sg := range out.SecurityGroups {
		if sg.GroupId == nil {
			continue
		}
		resources = append(resources, extract.RawCloudResource{
			ID:       fmt.Sprintf("aws:aws_security_group:%s", *sg.GroupId),
			Type:     "aws_security_group",
			Provider: "aws",
			Region:   region,
			CloudID:  *sg.GroupId,
			Attributes: map[string]any{
				"name":        aws.ToString(sg.GroupName),
				"description": aws.ToString(sg.Description),
				"vpc_id":      aws.ToString(sg.VpcId),
			},
			Tags:     tagsToMap(sg.Tags),
			Metadata: map[string]string{"source": "aws_api"},
		})
	}
	return resources, nil
}

func (p *Provider) fetchLambdaFunctions(ctx context.Context, cfg aws.Config, region string) ([]extract.RawCloudResource, error) {
	client := lambda.NewFromConfig(cfg)
	out, err := client.ListFunctions(ctx, &lambda.ListFunctionsInput{})
	if err != nil {
		return nil, err
	}
	var resources []extract.RawCloudResource
	for _, fn := range out.Functions {
		name := aws.ToString(fn.FunctionName)
		tags := map[string]string{}
		tagsOut, err := client.ListTags(ctx, &lambda.ListTagsInput{
			Resource: fn.FunctionArn,
		})
		if err == nil && tagsOut.Tags != nil {
			tags = tagsOut.Tags
		}
		resources = append(resources, extract.RawCloudResource{
			ID:       fmt.Sprintf("aws:aws_lambda_function:%s", name),
			Type:     "aws_lambda_function",
			Provider: "aws",
			Region:   region,
			CloudID:  name,
			Attributes: map[string]any{
				"function_name": name,
				"runtime":       string(fn.Runtime),
				"handler":       aws.ToString(fn.Handler),
				"memory_size":   aws.ToInt32(fn.MemorySize),
				"timeout":       aws.ToInt32(fn.Timeout),
				"role":          aws.ToString(fn.Role),
				"package_type":  string(fn.PackageType),
			},
			Tags:     tags,
			Metadata: map[string]string{"source": "aws_api"},
		})
	}
	return resources, nil
}

func tagsToMap(tags []ec2types.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		m[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return m
}

func sgIDs(sgs []ec2types.GroupIdentifier) []string {
	ids := make([]string, 0, len(sgs))
	for _, sg := range sgs {
		ids = append(ids, aws.ToString(sg.GroupId))
	}
	return ids
}
