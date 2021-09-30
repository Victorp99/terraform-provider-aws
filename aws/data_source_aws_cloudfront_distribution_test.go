package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/cloudfront"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
)

func TestAccAWSDataSourceCloudFrontDistribution_basic(t *testing.T) {
	dataSourceName := "data.aws_cloudfront_distribution.test"
	resourceName := "aws_cloudfront_distribution.s3_distribution"
	rInt := sdkacctest.RandInt()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:   func() { acctest.PreCheck(t); acctest.PreCheckPartitionHasService(cloudfront.EndpointsID, t) },
		ErrorCheck: acctest.ErrorCheck(t, cloudfront.EndpointsID),
		Providers:  acctest.Providers,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudFrontDistributionDataConfig(rInt),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, "arn", resourceName, "arn"),
					resource.TestCheckResourceAttrPair(dataSourceName, "domain_name", resourceName, "domain_name"),
					resource.TestCheckResourceAttrPair(dataSourceName, "etag", resourceName, "etag"),
					resource.TestCheckResourceAttrPair(dataSourceName, "hosted_zone_id", resourceName, "hosted_zone_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "in_progress_validation_batches", resourceName, "in_progress_validation_batches"),
					resource.TestCheckResourceAttrPair(dataSourceName, "last_modified_time", resourceName, "last_modified_time"),
					resource.TestCheckResourceAttrPair(dataSourceName, "status", resourceName, "status"),
				),
			},
		},
	})
}

func testAccAWSCloudFrontDistributionDataConfig(rInt int) string {
	return acctest.ConfigCompose(
		testAccAWSCloudFrontDistributionS3ConfigWithTags(rInt),
		`
data "aws_cloudfront_distribution" "test" {
  id = aws_cloudfront_distribution.s3_distribution.id
}
`)
}
