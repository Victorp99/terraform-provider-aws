package ec2

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceInternetGateway() *schema.Resource {
	return &schema.Resource{
		Create: resourceInternetGatewayCreate,
		Read:   resourceInternetGatewayRead,
		Update: resourceInternetGatewayUpdate,
		Delete: resourceInternetGatewayDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"vpc_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
			"owner_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceInternetGatewayCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	// Create the gateway
	log.Printf("[DEBUG] Creating internet gateway")
	var err error
	input := &ec2.CreateInternetGatewayInput{
		TagSpecifications: ec2TagSpecificationsFromKeyValueTags(tags, ec2.ResourceTypeInternetGateway),
	}
	resp, err := conn.CreateInternetGateway(input)
	if err != nil {
		return fmt.Errorf("Error creating internet gateway: %s", err)
	}

	// Get the ID and store it
	ig := *resp.InternetGateway
	d.SetId(aws.StringValue(ig.InternetGatewayId))
	log.Printf("[INFO] InternetGateway ID: %s", d.Id())
	var igRaw interface{}
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		igRaw, _, err = IGStateRefreshFunc(conn, d.Id())()
		if igRaw != nil {
			return nil
		}
		if err == nil {
			return resource.RetryableError(&resource.NotFoundError{})
		} else {
			return resource.NonRetryableError(err)
		}
	})
	if tfresource.TimedOut(err) {
		igRaw, _, err = IGStateRefreshFunc(conn, d.Id())()
		if igRaw == nil {
			return fmt.Errorf("error finding Internet Gateway (%s) after creation; retry running Terraform", d.Id())
		}
	}
	if err != nil {
		return fmt.Errorf("Error refreshing internet gateway state: %s", err)
	}

	// Attach the new gateway to the correct vpc
	err = resourceInternetGatewayAttach(d, meta)
	if err != nil {
		return fmt.Errorf("error attaching EC2 Internet Gateway (%s): %s", d.Id(), err)
	}

	return resourceInternetGatewayRead(d, meta)
}

func resourceInternetGatewayRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	igRaw, _, err := IGStateRefreshFunc(conn, d.Id())()
	if err != nil {
		return err
	}
	if igRaw == nil {
		log.Printf("[WARN] Internet Gateway (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	ig := igRaw.(*ec2.InternetGateway)
	if len(ig.Attachments) == 0 {
		// Gateway exists but not attached to the VPC
		d.Set("vpc_id", "")
	} else {
		d.Set("vpc_id", ig.Attachments[0].VpcId)
	}

	tags := KeyValueTags(ig.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	d.Set("owner_id", ig.OwnerId)

	arn := arn.ARN{
		Partition: meta.(*conns.AWSClient).Partition,
		Service:   ec2.ServiceName,
		Region:    meta.(*conns.AWSClient).Region,
		AccountID: aws.StringValue(ig.OwnerId),
		Resource:  fmt.Sprintf("internet-gateway/%s", d.Id()),
	}.String()

	d.Set("arn", arn)

	return nil
}

func resourceInternetGatewayUpdate(d *schema.ResourceData, meta interface{}) error {
	if d.HasChange("vpc_id") {
		// If we're already attached, detach it first
		if err := resourceInternetGatewayDetach(d, meta); err != nil {
			return err
		}

		// Attach the gateway to the new vpc
		if err := resourceInternetGatewayAttach(d, meta); err != nil {
			return err
		}
	}

	conn := meta.(*conns.AWSClient).EC2Conn

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return fmt.Errorf("error updating EC2 Internet Gateway (%s) tags: %s", d.Id(), err)
		}
	}

	return resourceInternetGatewayRead(d, meta)
}

func resourceInternetGatewayDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn

	// Detach if it is attached
	if err := resourceInternetGatewayDetach(d, meta); err != nil {
		return err
	}

	log.Printf("[INFO] Deleting Internet Gateway: %s", d.Id())
	input := &ec2.DeleteInternetGatewayInput{
		InternetGatewayId: aws.String(d.Id()),
	}
	err := resource.Retry(10*time.Minute, func() *resource.RetryError {
		_, err := conn.DeleteInternetGateway(input)
		if err == nil {
			return nil
		}

		if tfawserr.ErrMessageContains(err, "InvalidInternetGatewayID.NotFound", "") {
			return nil
		}

		if tfawserr.ErrMessageContains(err, "DependencyViolation", "") {
			return resource.RetryableError(err)
		}

		return resource.NonRetryableError(err)
	})
	if tfresource.TimedOut(err) {
		_, err = conn.DeleteInternetGateway(input)
	}
	if err != nil {
		return fmt.Errorf("Error deleting internet gateway: %s", err)
	}
	return nil
}

func resourceInternetGatewayAttach(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn

	if d.Get("vpc_id").(string) == "" {
		log.Printf(
			"[DEBUG] Not attaching Internet Gateway '%s' as no VPC ID is set",
			d.Id())
		return nil
	}

	log.Printf(
		"[INFO] Attaching Internet Gateway '%s' to VPC '%s'",
		d.Id(),
		d.Get("vpc_id").(string))
	input := &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(d.Id()),
		VpcId:             aws.String(d.Get("vpc_id").(string)),
	}
	err := resource.Retry(2*time.Minute, func() *resource.RetryError {
		_, err := conn.AttachInternetGateway(input)
		if err == nil {
			return nil
		}
		if tfawserr.ErrMessageContains(err, "InvalidInternetGatewayID.NotFound", "") {
			return resource.RetryableError(err)
		}

		return resource.NonRetryableError(err)
	})
	if tfresource.TimedOut(err) {
		_, err = conn.AttachInternetGateway(input)
	}
	if err != nil {
		return fmt.Errorf("Error attaching internet gateway: %s", err)
	}

	// A note on the states below: the AWS docs (as of July, 2014) say
	// that the states would be: attached, attaching, detached, detaching,
	// but when running, I noticed that the state is usually "available" when
	// it is attached.

	// Wait for it to be fully attached before continuing
	log.Printf("[DEBUG] Waiting for internet gateway (%s) to attach", d.Id())
	stateConf := &resource.StateChangeConf{
		Pending: []string{ec2.AttachmentStatusDetached, ec2.AttachmentStatusAttaching},
		Target:  []string{"available"},
		Refresh: IGAttachStateRefreshFunc(conn, d.Id(), "available"),
		Timeout: 4 * time.Minute,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf(
			"Error waiting for internet gateway (%s) to attach: %s",
			d.Id(), err)
	}

	return nil
}

func resourceInternetGatewayDetach(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).EC2Conn

	// Get the old VPC ID to detach from
	vpcID, _ := d.GetChange("vpc_id")

	if vpcID.(string) == "" {
		log.Printf(
			"[DEBUG] Not detaching Internet Gateway '%s' as no VPC ID is set",
			d.Id())
		return nil
	}

	log.Printf(
		"[INFO] Detaching Internet Gateway '%s' from VPC '%s'",
		d.Id(),
		vpcID.(string))

	// Wait for it to be fully detached before continuing
	log.Printf("[DEBUG] Waiting for internet gateway (%s) to detach", d.Id())
	stateConf := &resource.StateChangeConf{
		Pending:        []string{ec2.AttachmentStatusDetaching},
		Target:         []string{ec2.AttachmentStatusDetached},
		Refresh:        DetachIGStateRefreshFunc(conn, d.Id(), vpcID.(string)),
		Timeout:        15 * time.Minute,
		Delay:          10 * time.Second,
		NotFoundChecks: 30,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf(
			"Error waiting for internet gateway (%s) to detach: %s",
			d.Id(), err)
	}

	return nil
}

// InstanceStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch
// an EC2 instance.
func DetachIGStateRefreshFunc(conn *ec2.EC2, gatewayID, vpcID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		_, err := conn.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(gatewayID),
			VpcId:             aws.String(vpcID),
		})
		if err != nil {
			if ec2err, ok := err.(awserr.Error); ok {
				switch ec2err.Code() {
				case "InvalidInternetGatewayID.NotFound":
					log.Printf("[TRACE] Error detaching Internet Gateway '%s' from VPC '%s': %s", gatewayID, vpcID, err)
					return nil, "", nil

				case "Gateway.NotAttached":
					return 42, ec2.AttachmentStatusDetached, nil

				case "DependencyViolation":
					// This can be caused by associated public IPs left (e.g. by ELBs)
					// and here we find and log which ones are to blame
					out, err := findPublicNetworkInterfacesForVpcID(conn, vpcID)
					if err != nil {
						return 42, "detaching", err
					}
					if len(out.NetworkInterfaces) > 0 {
						log.Printf("[DEBUG] Waiting for the following %d ENIs to be gone: %s",
							len(out.NetworkInterfaces), out.NetworkInterfaces)
					}

					return 42, ec2.AttachmentStatusDetaching, nil
				}
			}
			return 42, "", err
		}

		// DetachInternetGateway only returns an error, so if it's nil, assume we're
		// detached
		return 42, ec2.AttachmentStatusDetached, nil
	}
}

func findPublicNetworkInterfacesForVpcID(conn *ec2.EC2, vpcID string) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return conn.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
			{
				Name:   aws.String("association.public-ip"),
				Values: []*string{aws.String("*")},
			},
		},
	})
}

// IGStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch
// an internet gateway.
func IGStateRefreshFunc(conn *ec2.EC2, id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
			InternetGatewayIds: []*string{aws.String(id)},
		})
		if err != nil {
			if tfawserr.ErrMessageContains(err, "InvalidInternetGatewayID.NotFound", "") {
				resp = nil
			} else {
				log.Printf("[ERROR] Error on IGStateRefresh: %s", err)
				return nil, "", err
			}
		}

		if resp == nil {
			// Sometimes AWS just has consistency issues and doesn't see
			// our instance yet. Return an empty state.
			return nil, "", nil
		}

		ig := resp.InternetGateways[0]
		return ig, "available", nil
	}
}

// IGAttachStateRefreshFunc returns a resource.StateRefreshFunc that is used
// watch the state of an internet gateway's attachment.
func IGAttachStateRefreshFunc(conn *ec2.EC2, id string, expected string) resource.StateRefreshFunc {
	var start time.Time
	return func() (interface{}, string, error) {
		if start.IsZero() {
			start = time.Now()
		}

		resp, err := conn.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
			InternetGatewayIds: []*string{aws.String(id)},
		})
		if err != nil {
			if tfawserr.ErrMessageContains(err, "InvalidInternetGatewayID.NotFound", "") {
				resp = nil
			} else {
				log.Printf("[ERROR] Error on IGStateRefresh: %s", err)
				return nil, "", err
			}
		}

		if resp == nil {
			// Sometimes AWS just has consistency issues and doesn't see
			// our instance yet. Return an empty state.
			return nil, "", nil
		}

		ig := resp.InternetGateways[0]

		if time.Since(start) > 10*time.Second {
			return ig, expected, nil
		}

		if len(ig.Attachments) == 0 {
			// No attachments, we're detached
			return ig, ec2.AttachmentStatusDetached, nil
		}

		return ig, *ig.Attachments[0].State, nil
	}
}
