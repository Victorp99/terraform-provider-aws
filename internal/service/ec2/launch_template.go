package ec2

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceLaunchTemplate() *schema.Resource {
	return &schema.Resource{
		Create: resourceLaunchTemplateCreate,
		Read:   resourceLaunchTemplateRead,
		Update: resourceLaunchTemplateUpdate,
		Delete: resourceLaunchTemplateDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{"name_prefix"},
				ValidateFunc:  verify.ValidLaunchTemplateName,
			},

			"name_prefix": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{"name"},
				ValidateFunc:  verify.ValidLaunchTemplateName,
			},

			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 255),
			},

			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"default_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"update_default_version"},
				ValidateFunc:  validation.IntAtLeast(1),
			},

			"update_default_version": {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{"default_version"},
			},

			"latest_version": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"block_device_mappings": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"device_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"no_device": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"virtual_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"ebs": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"delete_on_termination": {
										// Use TypeString to allow an "unspecified" value,
										// since TypeBool only has true/false with false default.
										// The conversion from bare true/false values in
										// configurations to TypeString value is currently safe.
										Type:             schema.TypeString,
										Optional:         true,
										DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
										ValidateFunc:     verify.ValidTypeStringNullableBoolean,
									},
									"encrypted": {
										// Use TypeString to allow an "unspecified" value,
										// since TypeBool only has true/false with false default.
										// The conversion from bare true/false values in
										// configurations to TypeString value is currently safe.
										Type:             schema.TypeString,
										Optional:         true,
										DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
										ValidateFunc:     verify.ValidTypeStringNullableBoolean,
									},
									"iops": {
										Type:     schema.TypeInt,
										Computed: true,
										Optional: true,
									},
									"kms_key_id": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: verify.ValidARN,
									},
									"snapshot_id": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"throughput": {
										Type:         schema.TypeInt,
										Computed:     true,
										Optional:     true,
										ValidateFunc: validation.IntBetween(125, 1000),
									},
									"volume_size": {
										Type:     schema.TypeInt,
										Optional: true,
										Computed: true,
									},
									"volume_type": {
										Type:         schema.TypeString,
										Optional:     true,
										Computed:     true,
										ValidateFunc: validation.StringInSlice(ec2.VolumeType_Values(), false),
									},
								},
							},
						},
					},
				},
			},

			"cpu_options": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"core_count": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"threads_per_core": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},

			"capacity_reservation_specification": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"capacity_reservation_preference": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(ec2.CapacityReservationPreference_Values(), false),
						},
						"capacity_reservation_target": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"capacity_reservation_id": {
										Type:     schema.TypeString,
										Optional: true,
									},
								},
							},
						},
					},
				},
			},

			"credit_specification": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cpu_credits": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(CPUCredits_Values(), false),
						},
					},
				},
			},

			"disable_api_termination": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"ebs_optimized": {
				// Use TypeString to allow an "unspecified" value,
				// since TypeBool only has true/false with false default.
				// The conversion from bare true/false values in
				// configurations to TypeString value is currently safe.
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
				ValidateFunc:     verify.ValidTypeStringNullableBoolean,
			},

			"elastic_gpu_specifications": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},

			"elastic_inference_accelerator": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},

			"iam_instance_profile": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"arn": {
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"iam_instance_profile.0.name"},
							ValidateFunc:  verify.ValidARN,
						},
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},

			"image_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"instance_initiated_shutdown_behavior": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(ec2.ShutdownBehavior_Values(), false),
			},

			"instance_market_options": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"market_type": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(ec2.MarketType_Values(), false),
						},
						"spot_options": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"block_duration_minutes": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntDivisibleBy(60),
									},
									"instance_interruption_behavior": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validation.StringInSlice(ec2.InstanceInterruptionBehavior_Values(), false),
									},
									"max_price": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"spot_instance_type": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validation.StringInSlice(ec2.SpotInstanceType_Values(), false),
									},
									"valid_until": {
										Type:         schema.TypeString,
										Optional:     true,
										Computed:     true,
										ValidateFunc: validation.IsRFC3339Time,
									},
								},
							},
						},
					},
				},
			},

			"instance_type": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"kernel_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"key_name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"license_specification": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"license_configuration_arn": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: verify.ValidARN,
						},
					},
				},
			},

			"metadata_options": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"http_endpoint": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.StringInSlice(ec2.LaunchTemplateInstanceMetadataEndpointState_Values(), false),
						},
						"http_protocol_ipv6": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled,
							ValidateFunc: validation.StringInSlice(ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values(), false),
						},
						"http_tokens": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.StringInSlice(ec2.LaunchTemplateHttpTokensState_Values(), false),
						},
						"http_put_response_hop_limit": {
							Type:         schema.TypeInt,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.IntBetween(1, 64),
						},
					},
				},
			},

			"enclave_options": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},

			"monitoring": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},

			"network_interfaces": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"associate_carrier_ip_address": {
							Type:             schema.TypeString,
							Optional:         true,
							DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
							ValidateFunc:     verify.ValidTypeStringNullableBoolean,
						},
						"associate_public_ip_address": {
							Type:             schema.TypeString,
							Optional:         true,
							DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
							ValidateFunc:     verify.ValidTypeStringNullableBoolean,
						},
						"delete_on_termination": {
							// Use TypeString to allow an "unspecified" value,
							// since TypeBool only has true/false with false default.
							// The conversion from bare true/false values in
							// configurations to TypeString value is currently safe.
							Type:             schema.TypeString,
							Optional:         true,
							DiffSuppressFunc: verify.SuppressEquivalentTypeStringBoolean,
							ValidateFunc:     verify.ValidTypeStringNullableBoolean,
						},
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"device_index": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"security_groups": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"ipv6_address_count": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"ipv6_addresses": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: validation.IsIPv6Address,
							},
						},
						"network_interface_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"private_ip_address": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.IsIPv4Address,
						},
						"ipv4_addresses": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: validation.IsIPv4Address,
							},
						},
						"ipv4_address_count": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"subnet_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"interface_type": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice([]string{"efa", "interface"}, false),
						},
					},
				},
			},

			"placement": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"affinity": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"availability_zone": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"group_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"host_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"host_resource_group_arn": {
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"placement.0.host_id"},
							ValidateFunc:  verify.ValidARN,
						},
						"spread_domain": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"tenancy": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateFunc: validation.StringInSlice([]string{
								ec2.TenancyDedicated,
								ec2.TenancyDefault,
								ec2.TenancyHost,
							}, false),
						},
						"partition_number": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},

			"ram_disk_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"security_group_names": {
				Type:          schema.TypeSet,
				Optional:      true,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"vpc_security_group_ids"},
			},

			"vpc_security_group_ids": {
				Type:          schema.TypeSet,
				Optional:      true,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"security_group_names"},
			},

			"tag_specifications": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"resource_type": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(ec2.ResourceType_Values(), false),
						},
						"tags": tftags.TagsSchema(),
					},
				},
			},

			"user_data": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
			"hibernation_options": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"configured": {
							Type:     schema.TypeBool,
							Required: true,
						},
					},
				},
			},
		},

		// Enable downstream updates for resources referencing schema attributes
		// to prevent non-empty plans after "terraform apply"
		CustomizeDiff: customdiff.Sequence(
			customdiff.ComputedIf("default_version", func(_ context.Context, diff *schema.ResourceDiff, meta interface{}) bool {
				for _, changedKey := range diff.GetChangedKeysPrefix("") {
					switch changedKey {
					case "name", "name_prefix", "description":
						continue
					default:
						return diff.Get("update_default_version").(bool)
					}
				}
				return false
			}),
			customdiff.ComputedIf("latest_version", func(_ context.Context, diff *schema.ResourceDiff, meta interface{}) bool {
				for _, changedKey := range diff.GetChangedKeysPrefix("") {
					switch changedKey {
					case "name", "name_prefix", "description", "default_version", "update_default_version":
						continue
					default:
						return true
					}
				}
				return false
			}),
			verify.SetTagsDiff,
		),
	}
}

func resourceLaunchTemplateCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	ltName := create.Name(d.Get("name").(string), d.Get("name_prefix").(string))

	launchTemplateData, err := buildLaunchTemplateData(d)
	if err != nil {
		return err
	}

	launchTemplateOpts := &ec2.CreateLaunchTemplateInput{
		ClientToken:        aws.String(resource.UniqueId()),
		LaunchTemplateName: aws.String(ltName),
		LaunchTemplateData: launchTemplateData,
		TagSpecifications:  ec2TagSpecificationsFromKeyValueTags(tags, ec2.ResourceTypeLaunchTemplate),
	}

	if v, ok := d.GetOk("description"); ok {
		launchTemplateOpts.VersionDescription = aws.String(v.(string))
	}

	resp, err := conn.CreateLaunchTemplate(launchTemplateOpts)
	if err != nil {
		return err
	}

	launchTemplate := resp.LaunchTemplate
	d.SetId(aws.StringValue(launchTemplate.LaunchTemplateId))

	log.Printf("[DEBUG] Launch Template created: %q (version %d)",
		*launchTemplate.LaunchTemplateId, *launchTemplate.LatestVersionNumber)

	return resourceLaunchTemplateRead(d, meta)
}

func resourceLaunchTemplateRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	log.Printf("[DEBUG] Reading launch template %s", d.Id())

	dlt, err := conn.DescribeLaunchTemplates(&ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateIds: []*string{aws.String(d.Id())},
	})

	if tfawserr.ErrMessageContains(err, ec2.LaunchTemplateErrorCodeLaunchTemplateIdDoesNotExist, "") {
		log.Printf("[WARN] launch template (%s) not found - removing from state", d.Id())
		d.SetId("")
		return nil
	}

	// AWS SDK constant above is currently incorrect
	if tfawserr.ErrMessageContains(err, "InvalidLaunchTemplateId.NotFound", "") {
		log.Printf("[WARN] launch template (%s) not found - removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("Error getting launch template: %s", err)
	}

	if dlt == nil || len(dlt.LaunchTemplates) == 0 {
		log.Printf("[WARN] launch template (%s) not found - removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if *dlt.LaunchTemplates[0].LaunchTemplateId != d.Id() {
		return fmt.Errorf("Unable to find launch template: %#v", dlt.LaunchTemplates)
	}

	log.Printf("[DEBUG] Found launch template %s", d.Id())

	lt := dlt.LaunchTemplates[0]
	d.Set("name", lt.LaunchTemplateName)
	d.Set("name_prefix", create.NamePrefixFromName(aws.StringValue(lt.LaunchTemplateName)))
	d.Set("latest_version", lt.LatestVersionNumber)
	d.Set("default_version", lt.DefaultVersionNumber)
	tags := KeyValueTags(lt.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	arn := arn.ARN{
		Partition: meta.(*conns.AWSClient).Partition,
		Service:   ec2.ServiceName,
		Region:    meta.(*conns.AWSClient).Region,
		AccountID: meta.(*conns.AWSClient).AccountID,
		Resource:  fmt.Sprintf("launch-template/%s", d.Id()),
	}.String()
	d.Set("arn", arn)

	version := strconv.Itoa(int(*lt.LatestVersionNumber))
	dltv, err := conn.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(d.Id()),
		Versions:         []*string{aws.String(version)},
	})
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Received launch template version %q (version %d)", d.Id(), *lt.LatestVersionNumber)

	d.Set("description", dltv.LaunchTemplateVersions[0].VersionDescription)

	ltData := dltv.LaunchTemplateVersions[0].LaunchTemplateData

	d.Set("disable_api_termination", ltData.DisableApiTermination)
	d.Set("image_id", ltData.ImageId)
	d.Set("instance_initiated_shutdown_behavior", ltData.InstanceInitiatedShutdownBehavior)
	d.Set("instance_type", ltData.InstanceType)
	d.Set("kernel_id", ltData.KernelId)
	d.Set("key_name", ltData.KeyName)
	d.Set("ram_disk_id", ltData.RamDiskId)
	d.Set("security_group_names", aws.StringValueSlice(ltData.SecurityGroups))
	d.Set("user_data", ltData.UserData)
	d.Set("vpc_security_group_ids", aws.StringValueSlice(ltData.SecurityGroupIds))
	d.Set("ebs_optimized", "")

	if ltData.EbsOptimized != nil {
		d.Set("ebs_optimized", strconv.FormatBool(aws.BoolValue(ltData.EbsOptimized)))
	}

	if err := d.Set("block_device_mappings", getBlockDeviceMappings(ltData.BlockDeviceMappings)); err != nil {
		return fmt.Errorf("error setting block_device_mappings: %s", err)
	}

	if err := d.Set("capacity_reservation_specification", getCapacityReservationSpecification(ltData.CapacityReservationSpecification)); err != nil {
		return fmt.Errorf("error setting capacity_reservation_specification: %s", err)
	}

	if err := d.Set("cpu_options", getCpuOptions(ltData.CpuOptions)); err != nil {
		return err
	}

	if strings.HasPrefix(aws.StringValue(ltData.InstanceType), "t2") || strings.HasPrefix(aws.StringValue(ltData.InstanceType), "t3") {
		if err := d.Set("credit_specification", getCreditSpecification(ltData.CreditSpecification)); err != nil {
			return fmt.Errorf("error setting credit_specification: %s", err)
		}
	}

	if err := d.Set("elastic_gpu_specifications", getElasticGpuSpecifications(ltData.ElasticGpuSpecifications)); err != nil {
		return fmt.Errorf("error setting elastic_gpu_specifications: %s", err)
	}

	if err := d.Set("elastic_inference_accelerator", flattenEc2LaunchTemplateElasticInferenceAcceleratorResponse(ltData.ElasticInferenceAccelerators)); err != nil {
		return fmt.Errorf("error setting elastic_inference_accelerator: %s", err)
	}

	if err := d.Set("iam_instance_profile", getIamInstanceProfile(ltData.IamInstanceProfile)); err != nil {
		return fmt.Errorf("error setting iam_instance_profile: %s", err)
	}

	if err := d.Set("instance_market_options", getInstanceMarketOptions(ltData.InstanceMarketOptions)); err != nil {
		return fmt.Errorf("error setting instance_market_options: %s", err)
	}

	if err := d.Set("license_specification", getLicenseSpecifications(ltData.LicenseSpecifications)); err != nil {
		return fmt.Errorf("error setting license_specification: %s", err)
	}

	if err := d.Set("metadata_options", flattenLaunchTemplateInstanceMetadataOptions(ltData.MetadataOptions)); err != nil {
		return fmt.Errorf("error setting metadata_options: %s", err)
	}

	if err := d.Set("enclave_options", getEnclaveOptions(ltData.EnclaveOptions)); err != nil {
		return fmt.Errorf("error setting enclave_options: %s", err)
	}

	if err := d.Set("monitoring", getMonitoring(ltData.Monitoring)); err != nil {
		return fmt.Errorf("error setting monitoring: %s", err)
	}

	if err := d.Set("network_interfaces", getNetworkInterfaces(ltData.NetworkInterfaces)); err != nil {
		return fmt.Errorf("error setting network_interfaces: %s", err)
	}

	if err := d.Set("placement", getPlacement(ltData.Placement)); err != nil {
		return fmt.Errorf("error setting placement: %s", err)
	}

	if err := d.Set("hibernation_options", flattenLaunchTemplateHibernationOptions(ltData.HibernationOptions)); err != nil {
		return fmt.Errorf("error setting hibernation_options: %s", err)
	}

	if err := d.Set("tag_specifications", getTagSpecifications(ltData.TagSpecifications)); err != nil {
		return fmt.Errorf("error setting tag_specifications: %s", err)
	}

	return nil
}

func resourceLaunchTemplateUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn

	latestVersion := int64(d.Get("latest_version").(int))
	defaultVersion := d.Get("default_version").(int)

	if d.HasChanges(updateKeys...) {
		launchTemplateData, err := buildLaunchTemplateData(d)
		if err != nil {
			return err
		}

		launchTemplateVersionOpts := &ec2.CreateLaunchTemplateVersionInput{
			ClientToken:        aws.String(resource.UniqueId()),
			LaunchTemplateId:   aws.String(d.Id()),
			LaunchTemplateData: launchTemplateData,
		}

		if v, ok := d.GetOk("description"); ok {
			launchTemplateVersionOpts.VersionDescription = aws.String(v.(string))
		}

		output, err := conn.CreateLaunchTemplateVersion(launchTemplateVersionOpts)
		if err != nil {
			return fmt.Errorf("error creating Launch Template (%s) Version: %w", d.Id(), err)
		}

		if output == nil || output.LaunchTemplateVersion == nil || output.LaunchTemplateVersion.VersionNumber == nil {
			return fmt.Errorf("error creating Launch Template (%s) Version: empty output", d.Id())
		}

		latestVersion = aws.Int64Value(output.LaunchTemplateVersion.VersionNumber)

	}

	if d.Get("update_default_version").(bool) || d.HasChange("default_version") {
		modifyLaunchTemplateOpts := &ec2.ModifyLaunchTemplateInput{
			LaunchTemplateId: aws.String(d.Id()),
		}
		if d.Get("update_default_version").(bool) {
			modifyLaunchTemplateOpts.DefaultVersion = aws.String(strconv.FormatInt(latestVersion, 10))
		} else if d.HasChange("default_version") {
			modifyLaunchTemplateOpts.DefaultVersion = aws.String(strconv.Itoa(defaultVersion))
		}

		_, err := conn.ModifyLaunchTemplate(modifyLaunchTemplateOpts)
		if err != nil {
			return fmt.Errorf("error modifying Launch Template (%s): %w", d.Id(), err)
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return fmt.Errorf("error updating tags: %s", err)
		}
	}

	return resourceLaunchTemplateRead(d, meta)
}

func resourceLaunchTemplateDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn

	log.Printf("[DEBUG] Launch Template destroy: %v", d.Id())
	_, err := conn.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{
		LaunchTemplateId: aws.String(d.Id()),
	})

	if tfawserr.ErrMessageContains(err, ec2.LaunchTemplateErrorCodeLaunchTemplateIdDoesNotExist, "") {
		return nil
	}
	// AWS SDK constant above is currently incorrect
	if tfawserr.ErrMessageContains(err, "InvalidLaunchTemplateId.NotFound", "") {
		return nil
	}
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Launch Template deleted: %v", d.Id())
	return nil
}

func getBlockDeviceMappings(m []*ec2.LaunchTemplateBlockDeviceMapping) []interface{} {
	s := []interface{}{}
	for _, v := range m {
		mapping := map[string]interface{}{
			"device_name":  aws.StringValue(v.DeviceName),
			"virtual_name": aws.StringValue(v.VirtualName),
		}
		if v.NoDevice != nil {
			mapping["no_device"] = aws.StringValue(v.NoDevice)
		}
		if v.Ebs != nil {
			ebs := map[string]interface{}{
				"volume_size": int(aws.Int64Value(v.Ebs.VolumeSize)),
				"volume_type": aws.StringValue(v.Ebs.VolumeType),
			}
			if v.Ebs.DeleteOnTermination != nil {
				ebs["delete_on_termination"] = strconv.FormatBool(aws.BoolValue(v.Ebs.DeleteOnTermination))
			}
			if v.Ebs.Encrypted != nil {
				ebs["encrypted"] = strconv.FormatBool(aws.BoolValue(v.Ebs.Encrypted))
			}
			if v.Ebs.Iops != nil {
				ebs["iops"] = aws.Int64Value(v.Ebs.Iops)
			}
			if v.Ebs.KmsKeyId != nil {
				ebs["kms_key_id"] = aws.StringValue(v.Ebs.KmsKeyId)
			}
			if v.Ebs.SnapshotId != nil {
				ebs["snapshot_id"] = aws.StringValue(v.Ebs.SnapshotId)
			}
			if v.Ebs.Throughput != nil {
				ebs["throughput"] = aws.Int64Value(v.Ebs.Throughput)
			}

			mapping["ebs"] = []interface{}{ebs}
		}
		s = append(s, mapping)
	}
	return s
}

func getCapacityReservationSpecification(crs *ec2.LaunchTemplateCapacityReservationSpecificationResponse) []interface{} {
	s := []interface{}{}
	if crs != nil {
		s = append(s, map[string]interface{}{
			"capacity_reservation_preference": aws.StringValue(crs.CapacityReservationPreference),
			"capacity_reservation_target":     getCapacityReservationTarget(crs.CapacityReservationTarget),
		})
	}
	return s
}

func getCapacityReservationTarget(crt *ec2.CapacityReservationTargetResponse) []interface{} {
	s := []interface{}{}
	if crt != nil {
		s = append(s, map[string]interface{}{
			"capacity_reservation_id": aws.StringValue(crt.CapacityReservationId),
		})
	}
	return s
}

func getCpuOptions(cs *ec2.LaunchTemplateCpuOptions) []interface{} {
	s := []interface{}{}
	if cs != nil {
		s = append(s, map[string]interface{}{
			"core_count":       aws.Int64Value(cs.CoreCount),
			"threads_per_core": aws.Int64Value(cs.ThreadsPerCore),
		})
	}
	return s
}

func getCreditSpecification(cs *ec2.CreditSpecification) []interface{} {
	s := []interface{}{}
	if cs != nil {
		s = append(s, map[string]interface{}{
			"cpu_credits": aws.StringValue(cs.CpuCredits),
		})
	}
	return s
}

func getElasticGpuSpecifications(e []*ec2.ElasticGpuSpecificationResponse) []interface{} {
	s := []interface{}{}
	for _, v := range e {
		s = append(s, map[string]interface{}{
			"type": aws.StringValue(v.Type),
		})
	}
	return s
}

func flattenEc2LaunchTemplateElasticInferenceAcceleratorResponse(accelerators []*ec2.LaunchTemplateElasticInferenceAcceleratorResponse) []interface{} {
	l := []interface{}{}

	for _, accelerator := range accelerators {
		if accelerator == nil {
			continue
		}

		m := map[string]interface{}{
			"type": aws.StringValue(accelerator.Type),
		}

		l = append(l, m)
	}

	return l
}

func getIamInstanceProfile(i *ec2.LaunchTemplateIamInstanceProfileSpecification) []interface{} {
	s := []interface{}{}
	if i != nil {
		s = append(s, map[string]interface{}{
			"arn":  aws.StringValue(i.Arn),
			"name": aws.StringValue(i.Name),
		})
	}
	return s
}

func getInstanceMarketOptions(m *ec2.LaunchTemplateInstanceMarketOptions) []interface{} {
	s := []interface{}{}
	if m != nil {
		mo := map[string]interface{}{
			"market_type": aws.StringValue(m.MarketType),
		}
		so := m.SpotOptions
		if so != nil {
			spotOptions := map[string]interface{}{}

			if so.BlockDurationMinutes != nil {
				spotOptions["block_duration_minutes"] = aws.Int64Value(so.BlockDurationMinutes)
			}

			if so.InstanceInterruptionBehavior != nil {
				spotOptions["instance_interruption_behavior"] = aws.StringValue(so.InstanceInterruptionBehavior)
			}

			if so.MaxPrice != nil {
				spotOptions["max_price"] = aws.StringValue(so.MaxPrice)
			}

			if so.SpotInstanceType != nil {
				spotOptions["spot_instance_type"] = aws.StringValue(so.SpotInstanceType)
			}

			if so.ValidUntil != nil {
				spotOptions["valid_until"] = aws.TimeValue(so.ValidUntil).Format(time.RFC3339)
			}

			mo["spot_options"] = []interface{}{spotOptions}
		}
		s = append(s, mo)
	}
	return s
}

func getLicenseSpecifications(licenseSpecifications []*ec2.LaunchTemplateLicenseConfiguration) []map[string]interface{} {
	var s []map[string]interface{}
	for _, v := range licenseSpecifications {
		s = append(s, map[string]interface{}{
			"license_configuration_arn": aws.StringValue(v.LicenseConfigurationArn),
		})
	}
	return s
}

func expandLaunchTemplateInstanceMetadataOptions(l []interface{}) *ec2.LaunchTemplateInstanceMetadataOptionsRequest {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	m := l[0].(map[string]interface{})

	opts := &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
		HttpEndpoint:     aws.String(m["http_endpoint"].(string)),
		HttpProtocolIpv6: aws.String(m["http_protocol_ipv6"].(string)),
	}

	if m["http_endpoint"].(string) == ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled {
		// These parameters are not allowed unless HttpEndpoint is enabled

		if v, ok := m["http_tokens"].(string); ok && v != "" {
			opts.HttpTokens = aws.String(v)
		}

		if v, ok := m["http_put_response_hop_limit"].(int); ok && v != 0 {
			opts.HttpPutResponseHopLimit = aws.Int64(int64(v))
		}
	}

	return opts
}

func flattenLaunchTemplateInstanceMetadataOptions(opts *ec2.LaunchTemplateInstanceMetadataOptions) []interface{} {
	if opts == nil {
		return nil
	}

	m := map[string]interface{}{
		"http_endpoint":               aws.StringValue(opts.HttpEndpoint),
		"http_protocol_ipv6":          aws.StringValue(opts.HttpProtocolIpv6),
		"http_put_response_hop_limit": aws.Int64Value(opts.HttpPutResponseHopLimit),
		"http_tokens":                 aws.StringValue(opts.HttpTokens),
	}

	return []interface{}{m}
}

func getEnclaveOptions(m *ec2.LaunchTemplateEnclaveOptions) []interface{} {
	s := []interface{}{}
	if m != nil {
		mo := map[string]interface{}{
			"enabled": aws.BoolValue(m.Enabled),
		}
		s = append(s, mo)
	}
	return s
}

func getMonitoring(m *ec2.LaunchTemplatesMonitoring) []interface{} {
	s := []interface{}{}
	if m != nil {
		mo := map[string]interface{}{
			"enabled": aws.BoolValue(m.Enabled),
		}
		s = append(s, mo)
	}
	return s
}

func getNetworkInterfaces(n []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecification) []interface{} {
	s := []interface{}{}
	for _, v := range n {
		var ipv4Addresses []string

		networkInterface := map[string]interface{}{
			"description":          aws.StringValue(v.Description),
			"device_index":         aws.Int64Value(v.DeviceIndex),
			"interface_type":       aws.StringValue(v.InterfaceType),
			"ipv4_address_count":   aws.Int64Value(v.SecondaryPrivateIpAddressCount),
			"ipv6_address_count":   aws.Int64Value(v.Ipv6AddressCount),
			"network_interface_id": aws.StringValue(v.NetworkInterfaceId),
			"private_ip_address":   aws.StringValue(v.PrivateIpAddress),
			"subnet_id":            aws.StringValue(v.SubnetId),
		}

		if v.AssociateCarrierIpAddress != nil {
			networkInterface["associate_carrier_ip_address"] = strconv.FormatBool(aws.BoolValue(v.AssociateCarrierIpAddress))
		}

		if v.AssociatePublicIpAddress != nil {
			networkInterface["associate_public_ip_address"] = strconv.FormatBool(aws.BoolValue(v.AssociatePublicIpAddress))
		}

		if v.DeleteOnTermination != nil {
			networkInterface["delete_on_termination"] = strconv.FormatBool(aws.BoolValue(v.DeleteOnTermination))
		}

		if len(v.Ipv6Addresses) > 0 {
			raw, ok := networkInterface["ipv6_addresses"]
			if !ok {
				raw = schema.NewSet(schema.HashString, nil)
			}

			list := raw.(*schema.Set)

			for _, address := range v.Ipv6Addresses {
				list.Add(aws.StringValue(address.Ipv6Address))
			}

			networkInterface["ipv6_addresses"] = list
		}

		for _, address := range v.PrivateIpAddresses {
			ipv4Addresses = append(ipv4Addresses, aws.StringValue(address.PrivateIpAddress))
		}
		if len(ipv4Addresses) > 0 {
			networkInterface["ipv4_addresses"] = ipv4Addresses
		}

		if len(v.Groups) > 0 {
			raw, ok := networkInterface["security_groups"]
			if !ok {
				raw = schema.NewSet(schema.HashString, nil)
			}
			list := raw.(*schema.Set)

			for _, group := range v.Groups {
				list.Add(aws.StringValue(group))
			}

			networkInterface["security_groups"] = list
		}

		s = append(s, networkInterface)
	}
	return s
}

func getPlacement(p *ec2.LaunchTemplatePlacement) []interface{} {
	var s []interface{}
	if p != nil {
		s = append(s, map[string]interface{}{
			"affinity":                aws.StringValue(p.Affinity),
			"availability_zone":       aws.StringValue(p.AvailabilityZone),
			"group_name":              aws.StringValue(p.GroupName),
			"host_id":                 aws.StringValue(p.HostId),
			"host_resource_group_arn": aws.StringValue(p.HostResourceGroupArn),
			"spread_domain":           aws.StringValue(p.SpreadDomain),
			"tenancy":                 aws.StringValue(p.Tenancy),
			"partition_number":        aws.Int64Value(p.PartitionNumber),
		})
	}
	return s
}

func expandLaunchTemplateHibernationOptions(l []interface{}) *ec2.LaunchTemplateHibernationOptionsRequest {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	m := l[0].(map[string]interface{})

	opts := &ec2.LaunchTemplateHibernationOptionsRequest{
		Configured: aws.Bool(m["configured"].(bool)),
	}

	return opts
}

func flattenLaunchTemplateHibernationOptions(m *ec2.LaunchTemplateHibernationOptions) []interface{} {
	s := []interface{}{}
	if m != nil {
		mo := map[string]interface{}{
			"configured": aws.BoolValue(m.Configured),
		}
		s = append(s, mo)
	}
	return s
}

func getTagSpecifications(t []*ec2.LaunchTemplateTagSpecification) []interface{} {
	var s []interface{}
	for _, v := range t {
		s = append(s, map[string]interface{}{
			"resource_type": aws.StringValue(v.ResourceType),
			"tags":          KeyValueTags(v.Tags).IgnoreAWS().Map(),
		})
	}
	return s
}

func buildLaunchTemplateData(d *schema.ResourceData) (*ec2.RequestLaunchTemplateData, error) {
	opts := &ec2.RequestLaunchTemplateData{
		UserData: aws.String(d.Get("user_data").(string)),
	}

	if v, ok := d.GetOk("image_id"); ok {
		opts.ImageId = aws.String(v.(string))
	}

	if v, ok := d.GetOk("instance_initiated_shutdown_behavior"); ok {
		opts.InstanceInitiatedShutdownBehavior = aws.String(v.(string))
	}

	instanceType := d.Get("instance_type").(string)
	if instanceType != "" {
		opts.InstanceType = aws.String(instanceType)
	}

	if v, ok := d.GetOk("kernel_id"); ok {
		opts.KernelId = aws.String(v.(string))
	}

	if v, ok := d.GetOk("key_name"); ok {
		opts.KeyName = aws.String(v.(string))
	}

	if v, ok := d.GetOk("ram_disk_id"); ok {
		opts.RamDiskId = aws.String(v.(string))
	}

	if v, ok := d.GetOk("disable_api_termination"); ok {
		opts.DisableApiTermination = aws.Bool(v.(bool))
	}

	if v, ok := d.GetOk("ebs_optimized"); ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting ebs_optimized %q from string to boolean: %s", v.(string), err)
		}
		opts.EbsOptimized = aws.Bool(vBool)
	}

	if v, ok := d.GetOk("security_group_names"); ok {
		opts.SecurityGroups = flex.ExpandStringSet(v.(*schema.Set))
	}

	if v, ok := d.GetOk("vpc_security_group_ids"); ok {
		opts.SecurityGroupIds = flex.ExpandStringSet(v.(*schema.Set))
	}

	if v, ok := d.GetOk("block_device_mappings"); ok {
		var blockDeviceMappings []*ec2.LaunchTemplateBlockDeviceMappingRequest
		bdms := v.([]interface{})

		for _, bdm := range bdms {
			if bdm == nil {
				continue
			}
			blockDeviceMapping, err := readBlockDeviceMappingFromConfig(bdm.(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			blockDeviceMappings = append(blockDeviceMappings, blockDeviceMapping)
		}
		opts.BlockDeviceMappings = blockDeviceMappings
	}

	if v, ok := d.GetOk("capacity_reservation_specification"); ok {
		crs := v.([]interface{})

		if len(crs) > 0 && crs[0] != nil {
			opts.CapacityReservationSpecification = readCapacityReservationSpecificationFromConfig(crs[0].(map[string]interface{}))
		}
	}

	if v, ok := d.GetOk("cpu_options"); ok {
		co := v.([]interface{})

		if len(co) > 0 {
			opts.CpuOptions = readCpuOptionsFromConfig(co[0].(map[string]interface{}))
		}
	}

	if v, ok := d.GetOk("credit_specification"); ok && (strings.HasPrefix(instanceType, "t2") || strings.HasPrefix(instanceType, "t3")) {
		cs := v.([]interface{})

		if len(cs) > 0 && cs[0] != nil {
			opts.CreditSpecification = readCreditSpecificationFromConfig(cs[0].(map[string]interface{}))
		}
	}

	if v, ok := d.GetOk("elastic_gpu_specifications"); ok {
		var elasticGpuSpecifications []*ec2.ElasticGpuSpecification
		egsList := v.([]interface{})

		for _, egs := range egsList {
			elasticGpuSpecifications = append(elasticGpuSpecifications, readElasticGpuSpecificationsFromConfig(egs.(map[string]interface{})))
		}
		opts.ElasticGpuSpecifications = elasticGpuSpecifications
	}

	if v, ok := d.GetOk("elastic_inference_accelerator"); ok {
		opts.ElasticInferenceAccelerators = expandEc2LaunchTemplateElasticInferenceAccelerators(v.([]interface{}))
	}

	if v, ok := d.GetOk("iam_instance_profile"); ok {
		iip := v.([]interface{})

		if len(iip) > 0 && iip[0] != nil {
			opts.IamInstanceProfile = readIamInstanceProfileFromConfig(iip[0].(map[string]interface{}))
		}
	}

	if v, ok := d.GetOk("instance_market_options"); ok {
		imo := v.([]interface{})

		if len(imo) > 0 && imo[0] != nil {
			instanceMarketOptions, err := readInstanceMarketOptionsFromConfig(imo[0].(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			opts.InstanceMarketOptions = instanceMarketOptions
		}
	}

	if v, ok := d.GetOk("license_specification"); ok {
		var licenseSpecifications []*ec2.LaunchTemplateLicenseConfigurationRequest
		lsList := v.(*schema.Set).List()

		for _, ls := range lsList {
			if ls == nil {
				continue
			}
			licenseSpecifications = append(licenseSpecifications, readLicenseSpecificationFromConfig(ls.(map[string]interface{})))
		}
		opts.LicenseSpecifications = licenseSpecifications
	}

	if v, ok := d.GetOk("metadata_options"); ok {
		opts.MetadataOptions = expandLaunchTemplateInstanceMetadataOptions(v.([]interface{}))
	}

	if v, ok := d.GetOk("enclave_options"); ok {
		m := v.([]interface{})
		if len(m) > 0 && m[0] != nil {
			mData := m[0].(map[string]interface{})
			enclaveOptions := &ec2.LaunchTemplateEnclaveOptionsRequest{
				Enabled: aws.Bool(mData["enabled"].(bool)),
			}
			opts.EnclaveOptions = enclaveOptions
		}
	}

	if v, ok := d.GetOk("monitoring"); ok {
		m := v.([]interface{})
		if len(m) > 0 && m[0] != nil {
			mData := m[0].(map[string]interface{})
			monitoring := &ec2.LaunchTemplatesMonitoringRequest{
				Enabled: aws.Bool(mData["enabled"].(bool)),
			}
			opts.Monitoring = monitoring
		}
	}

	if v, ok := d.GetOk("network_interfaces"); ok {
		var networkInterfaces []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest
		niList := v.([]interface{})

		for _, ni := range niList {
			if ni == nil {
				continue
			}
			niData := ni.(map[string]interface{})
			networkInterface, err := readNetworkInterfacesFromConfig(niData)
			if err != nil {
				return nil, err
			}
			networkInterfaces = append(networkInterfaces, networkInterface)
		}
		opts.NetworkInterfaces = networkInterfaces
	}

	if v, ok := d.GetOk("placement"); ok {
		p := v.([]interface{})

		if len(p) > 0 && p[0] != nil {
			opts.Placement = readPlacementFromConfig(p[0].(map[string]interface{}))
		}
	}

	if v, ok := d.GetOk("hibernation_options"); ok {
		opts.HibernationOptions = expandLaunchTemplateHibernationOptions(v.([]interface{}))
	}

	if v, ok := d.GetOk("tag_specifications"); ok {
		var tagSpecifications []*ec2.LaunchTemplateTagSpecificationRequest
		t := v.([]interface{})

		for _, ts := range t {
			if ts == nil {
				continue
			}
			tsData := ts.(map[string]interface{})
			tagSpecification := &ec2.LaunchTemplateTagSpecificationRequest{
				ResourceType: aws.String(tsData["resource_type"].(string)),
				Tags:         Tags(tftags.New(tsData["tags"].(map[string]interface{})).IgnoreAWS()),
			}
			tagSpecifications = append(tagSpecifications, tagSpecification)
		}
		opts.TagSpecifications = tagSpecifications
	}

	return opts, nil
}

func readBlockDeviceMappingFromConfig(bdm map[string]interface{}) (*ec2.LaunchTemplateBlockDeviceMappingRequest, error) {
	blockDeviceMapping := &ec2.LaunchTemplateBlockDeviceMappingRequest{}

	if v := bdm["device_name"].(string); v != "" {
		blockDeviceMapping.DeviceName = aws.String(v)
	}

	if v := bdm["no_device"].(string); v != "" {
		blockDeviceMapping.NoDevice = aws.String(v)
	}

	if v := bdm["virtual_name"].(string); v != "" {
		blockDeviceMapping.VirtualName = aws.String(v)
	}

	if v := bdm["ebs"]; len(v.([]interface{})) > 0 {
		ebs := v.([]interface{})
		if len(ebs) > 0 && ebs[0] != nil {
			ebsData := ebs[0].(map[string]interface{})
			launchTemplateEbsBlockDeviceRequest, err := readEbsBlockDeviceFromConfig(ebsData)
			if err != nil {
				return nil, err
			}
			blockDeviceMapping.Ebs = launchTemplateEbsBlockDeviceRequest
		}
	}

	return blockDeviceMapping, nil
}

func readEbsBlockDeviceFromConfig(ebs map[string]interface{}) (*ec2.LaunchTemplateEbsBlockDeviceRequest, error) {
	ebsDevice := &ec2.LaunchTemplateEbsBlockDeviceRequest{}

	if v, ok := ebs["delete_on_termination"]; ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting delete_on_termination %q from string to boolean: %s", v.(string), err)
		}
		ebsDevice.DeleteOnTermination = aws.Bool(vBool)
	}

	if v, ok := ebs["encrypted"]; ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting encrypted %q from string to boolean: %s", v.(string), err)
		}
		ebsDevice.Encrypted = aws.Bool(vBool)
	}

	if v := ebs["iops"].(int); v > 0 {
		ebsDevice.Iops = aws.Int64(int64(v))
	}

	if v := ebs["kms_key_id"].(string); v != "" {
		ebsDevice.KmsKeyId = aws.String(v)
	}

	if v := ebs["snapshot_id"].(string); v != "" {
		ebsDevice.SnapshotId = aws.String(v)
	}

	if v := ebs["throughput"].(int); v > 0 {
		ebsDevice.Throughput = aws.Int64(int64(v))
	}

	if v := ebs["volume_size"]; v != nil {
		ebsDevice.VolumeSize = aws.Int64(int64(v.(int)))
	}

	if v := ebs["volume_type"].(string); v != "" {
		ebsDevice.VolumeType = aws.String(v)
	}

	return ebsDevice, nil
}

func readNetworkInterfacesFromConfig(ni map[string]interface{}) (*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest, error) {
	var ipv4Addresses []*ec2.PrivateIpAddressSpecification
	var ipv6Addresses []*ec2.InstanceIpv6AddressRequest
	var privateIpAddress string
	networkInterface := &ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{}

	if v, ok := ni["delete_on_termination"]; ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting delete_on_termination %q from string to boolean: %w", v.(string), err)
		}
		networkInterface.DeleteOnTermination = aws.Bool(vBool)
	}

	if v, ok := ni["description"].(string); ok && v != "" {
		networkInterface.Description = aws.String(v)
	}

	if v, ok := ni["device_index"].(int); ok {
		networkInterface.DeviceIndex = aws.Int64(int64(v))
	}

	if v, ok := ni["network_interface_id"].(string); ok && v != "" {
		networkInterface.NetworkInterfaceId = aws.String(v)
	}

	if v, ok := ni["interface_type"].(string); ok && v != "" {
		networkInterface.InterfaceType = aws.String(v)
	}

	if v, ok := ni["associate_carrier_ip_address"]; ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting associate_carrier_ip_address %q from string to boolean: %s", v.(string), err)
		}
		networkInterface.AssociateCarrierIpAddress = aws.Bool(vBool)
	}

	if v, ok := ni["associate_public_ip_address"]; ok && v.(string) != "" {
		vBool, err := strconv.ParseBool(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error converting associate_public_ip_address %q from string to boolean: %s", v.(string), err)
		}
		networkInterface.AssociatePublicIpAddress = aws.Bool(vBool)
	}

	if v, ok := ni["private_ip_address"].(string); ok && v != "" {
		privateIpAddress = v
		networkInterface.PrivateIpAddress = aws.String(v)
	}

	if v, ok := ni["subnet_id"].(string); ok && v != "" {
		networkInterface.SubnetId = aws.String(v)
	}

	if v := ni["security_groups"].(*schema.Set); v.Len() > 0 {
		for _, v := range v.List() {
			networkInterface.Groups = append(networkInterface.Groups, aws.String(v.(string)))
		}
	}

	ipv6AddressList := ni["ipv6_addresses"].(*schema.Set).List()
	for _, address := range ipv6AddressList {
		ipv6Addresses = append(ipv6Addresses, &ec2.InstanceIpv6AddressRequest{
			Ipv6Address: aws.String(address.(string)),
		})
	}
	networkInterface.Ipv6Addresses = ipv6Addresses

	if v := ni["ipv6_address_count"].(int); v > 0 {
		networkInterface.Ipv6AddressCount = aws.Int64(int64(v))
	}

	if v := ni["ipv4_address_count"].(int); v > 0 {
		networkInterface.SecondaryPrivateIpAddressCount = aws.Int64(int64(v))
	} else if v := ni["ipv4_addresses"].(*schema.Set); v.Len() > 0 {
		for _, address := range v.List() {
			privateIp := &ec2.PrivateIpAddressSpecification{
				Primary:          aws.Bool(address.(string) == privateIpAddress),
				PrivateIpAddress: aws.String(address.(string)),
			}
			ipv4Addresses = append(ipv4Addresses, privateIp)
		}
		networkInterface.PrivateIpAddresses = ipv4Addresses
	}

	return networkInterface, nil
}

func readIamInstanceProfileFromConfig(iip map[string]interface{}) *ec2.LaunchTemplateIamInstanceProfileSpecificationRequest {
	iamInstanceProfile := &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{}

	if v, ok := iip["arn"].(string); ok && v != "" {
		iamInstanceProfile.Arn = aws.String(v)
	}

	if v, ok := iip["name"].(string); ok && v != "" {
		iamInstanceProfile.Name = aws.String(v)
	}

	return iamInstanceProfile
}

func readCapacityReservationSpecificationFromConfig(crs map[string]interface{}) *ec2.LaunchTemplateCapacityReservationSpecificationRequest {
	capacityReservationSpecification := &ec2.LaunchTemplateCapacityReservationSpecificationRequest{}

	if v, ok := crs["capacity_reservation_preference"].(string); ok && v != "" {
		capacityReservationSpecification.CapacityReservationPreference = aws.String(v)
	}

	if v, ok := crs["capacity_reservation_target"]; ok {
		crt := v.([]interface{})

		if len(crt) > 0 {
			capacityReservationSpecification.CapacityReservationTarget = readCapacityReservationTargetFromConfig(crt[0].(map[string]interface{}))
		}
	}

	return capacityReservationSpecification
}

func readCapacityReservationTargetFromConfig(crt map[string]interface{}) *ec2.CapacityReservationTarget {
	capacityReservationTarget := &ec2.CapacityReservationTarget{}

	if v, ok := crt["capacity_reservation_id"].(string); ok && v != "" {
		capacityReservationTarget.CapacityReservationId = aws.String(v)
	}

	return capacityReservationTarget
}

func readCpuOptionsFromConfig(co map[string]interface{}) *ec2.LaunchTemplateCpuOptionsRequest {
	cpuOptions := &ec2.LaunchTemplateCpuOptionsRequest{}

	if v, ok := co["core_count"].(int); ok && v != 0 {
		cpuOptions.CoreCount = aws.Int64(int64(v))
	}

	if v, ok := co["threads_per_core"].(int); ok && v != 0 {
		cpuOptions.ThreadsPerCore = aws.Int64(int64(v))
	}

	return cpuOptions
}

func readCreditSpecificationFromConfig(cs map[string]interface{}) *ec2.CreditSpecificationRequest {
	creditSpecification := &ec2.CreditSpecificationRequest{}

	if v, ok := cs["cpu_credits"].(string); ok && v != "" {
		creditSpecification.CpuCredits = aws.String(v)
	}

	return creditSpecification
}

func readElasticGpuSpecificationsFromConfig(egs map[string]interface{}) *ec2.ElasticGpuSpecification {
	elasticGpuSpecification := &ec2.ElasticGpuSpecification{}

	if v, ok := egs["type"].(string); ok && v != "" {
		elasticGpuSpecification.Type = aws.String(v)
	}

	return elasticGpuSpecification
}

func expandEc2LaunchTemplateElasticInferenceAccelerators(l []interface{}) []*ec2.LaunchTemplateElasticInferenceAccelerator {
	if len(l) == 0 {
		return nil
	}

	var accelerators []*ec2.LaunchTemplateElasticInferenceAccelerator

	for _, lRaw := range l {
		m, ok := lRaw.(map[string]interface{})

		if !ok {
			continue
		}

		accelerator := &ec2.LaunchTemplateElasticInferenceAccelerator{
			Type: aws.String(m["type"].(string)),
		}

		accelerators = append(accelerators, accelerator)
	}

	return accelerators
}

func readInstanceMarketOptionsFromConfig(imo map[string]interface{}) (*ec2.LaunchTemplateInstanceMarketOptionsRequest, error) {
	instanceMarketOptions := &ec2.LaunchTemplateInstanceMarketOptionsRequest{}
	spotOptions := &ec2.LaunchTemplateSpotMarketOptionsRequest{}

	if v, ok := imo["market_type"].(string); ok && v != "" {
		instanceMarketOptions.MarketType = aws.String(v)
	}

	if v, ok := imo["spot_options"]; ok {
		vL := v.([]interface{})
		for _, v := range vL {
			so, ok := v.(map[string]interface{})
			if !ok {
				continue
			}

			if v, ok := so["block_duration_minutes"].(int); ok && v != 0 {
				spotOptions.BlockDurationMinutes = aws.Int64(int64(v))
			}

			if v, ok := so["instance_interruption_behavior"].(string); ok && v != "" {
				spotOptions.InstanceInterruptionBehavior = aws.String(v)
			}

			if v, ok := so["max_price"].(string); ok && v != "" {
				spotOptions.MaxPrice = aws.String(v)
			}

			if v, ok := so["spot_instance_type"].(string); ok && v != "" {
				spotOptions.SpotInstanceType = aws.String(v)
			}

			if v, ok := so["valid_until"].(string); ok && v != "" {
				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return nil, fmt.Errorf("Error Parsing Launch Template Spot Options valid until: %s", err.Error())
				}
				spotOptions.ValidUntil = aws.Time(t)
			}
		}
		instanceMarketOptions.SpotOptions = spotOptions
	}

	return instanceMarketOptions, nil
}

func readLicenseSpecificationFromConfig(ls map[string]interface{}) *ec2.LaunchTemplateLicenseConfigurationRequest {
	licenseSpecification := &ec2.LaunchTemplateLicenseConfigurationRequest{}

	if v, ok := ls["license_configuration_arn"].(string); ok && v != "" {
		licenseSpecification.LicenseConfigurationArn = aws.String(v)
	}

	return licenseSpecification
}

func readPlacementFromConfig(p map[string]interface{}) *ec2.LaunchTemplatePlacementRequest {
	placement := &ec2.LaunchTemplatePlacementRequest{}

	if v, ok := p["affinity"].(string); ok && v != "" {
		placement.Affinity = aws.String(v)
	}

	if v, ok := p["availability_zone"].(string); ok && v != "" {
		placement.AvailabilityZone = aws.String(v)
	}

	if v, ok := p["group_name"].(string); ok && v != "" {
		placement.GroupName = aws.String(v)
	}

	if v, ok := p["host_id"].(string); ok && v != "" {
		placement.HostId = aws.String(v)
	}

	if v, ok := p["host_resource_group_arn"].(string); ok && v != "" {
		placement.HostResourceGroupArn = aws.String(v)
	}

	if v, ok := p["spread_domain"].(string); ok && v != "" {
		placement.SpreadDomain = aws.String(v)
	}

	if v, ok := p["tenancy"].(string); ok && v != "" {
		placement.Tenancy = aws.String(v)
	}

	if v, ok := p["partition_number"].(int); ok && v != 0 {
		placement.PartitionNumber = aws.Int64(int64(v))
	}

	return placement
}

var updateKeys = []string{
	"block_device_mappings",
	"capacity_reservation_specification",
	"cpu_options",
	"credit_specification",
	"description",
	"disable_api_termination",
	"ebs_optimized",
	"elastic_gpu_specifications",
	"elastic_inference_accelerator",
	"enclave_options",
	"hibernation_options",
	"iam_instance_profile",
	"image_id",
	"instance_initiated_shutdown_behavior",
	"instance_market_options",
	"instance_type",
	"kernel_id",
	"key_name",
	"license_specification",
	"metadata_options",
	"monitoring",
	"network_interfaces",
	"placement",
	"ram_disk_id",
	"security_group_names",
	"tag_specifications",
	"user_data",
	"vpc_security_group_ids",
}
