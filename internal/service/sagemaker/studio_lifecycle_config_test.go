package sagemaker_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/hashicorp/go-multierror"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfsagemaker "github.com/hashicorp/terraform-provider-aws/internal/service/sagemaker"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func init() {
	resource.AddTestSweepers("aws_sagemaker_studio_lifecycle_config", &resource.Sweeper{
		Name: "aws_sagemaker_studio_lifecycle_config",
		F:    sweepStudioLifecyclesConfig,
		Dependencies: []string{
			"aws_sagemaker_domain",
		},
	})
}

func sweepStudioLifecyclesConfig(region string) error {
	client, err := sweep.SharedRegionalSweepClient(region)
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}
	conn := client.(*conns.AWSClient).SageMakerConn
	var sweeperErrs *multierror.Error

	err = conn.ListStudioLifecycleConfigsPages(&sagemaker.ListStudioLifecycleConfigsInput{}, func(page *sagemaker.ListStudioLifecycleConfigsOutput, lastPage bool) bool {
		for _, config := range page.StudioLifecycleConfigs {

			r := tfsagemaker.ResourceStudioLifecycleConfig()
			d := r.Data(nil)
			d.SetId(aws.StringValue(config.StudioLifecycleConfigName))
			err := r.Delete(d, client)
			if err != nil {
				log.Printf("[ERROR] %s", err)
				sweeperErrs = multierror.Append(sweeperErrs, err)
				continue
			}
		}

		return !lastPage
	})

	if sweep.SkipSweepError(err) {
		log.Printf("[WARN] Skipping SageMaker Studio Lifecycle Config sweep for %s: %s", region, err)
		return sweeperErrs.ErrorOrNil()
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error retrieving Sagemaker Studio Lifecycle Configs: %w", err))
	}

	return sweeperErrs.ErrorOrNil()
}

func TestAccSageMakerStudioLifecycleConfig_basic(t *testing.T) {
	var config sagemaker.DescribeStudioLifecycleConfigOutput
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_sagemaker_studio_lifecycle_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, sagemaker.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStudioLifecycleDestroyConfig,
		Steps: []resource.TestStep{
			{
				Config: testAccStudioLifecycleBasicConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStudioLifecycleExistsConfig(resourceName, &config),
					resource.TestCheckResourceAttr(resourceName, "studio_lifecycle_config_name", rName),
					acctest.CheckResourceAttrRegionalARN(resourceName, "arn", "sagemaker", fmt.Sprintf("studio-lifecycle-config/%s", rName)),
					resource.TestCheckResourceAttr(resourceName, "studio_lifecycle_config_app_type", "JupyterServer"),
					resource.TestCheckResourceAttrSet(resourceName, "studio_lifecycle_config_content"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSageMakerStudioLifecycleConfig_tags(t *testing.T) {
	var config sagemaker.DescribeStudioLifecycleConfigOutput
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_sagemaker_studio_lifecycle_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, sagemaker.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStudioLifecycleDestroyConfig,
		Steps: []resource.TestStep{
			{
				Config: testAccStudioLifecycleTags1Config(rName, "key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStudioLifecycleExistsConfig(resourceName, &config),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccStudioLifecycleTags2Config(rName, "key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStudioLifecycleExistsConfig(resourceName, &config),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
			{
				Config: testAccStudioLifecycleTags1Config(rName, "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStudioLifecycleExistsConfig(resourceName, &config),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
		},
	})
}

func TestAccSageMakerStudioLifecycleConfig_disappears(t *testing.T) {
	var config sagemaker.DescribeStudioLifecycleConfigOutput
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_sagemaker_studio_lifecycle_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, sagemaker.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStudioLifecycleDestroyConfig,
		Steps: []resource.TestStep{
			{
				Config: testAccStudioLifecycleBasicConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStudioLifecycleExistsConfig(resourceName, &config),
					acctest.CheckResourceDisappears(acctest.Provider, tfsagemaker.ResourceStudioLifecycleConfig(), resourceName),
					acctest.CheckResourceDisappears(acctest.Provider, tfsagemaker.ResourceStudioLifecycleConfig(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckStudioLifecycleDestroyConfig(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).SageMakerConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_sagemaker_studio_lifecycle_config" {
			continue
		}

		_, err := tfsagemaker.FindStudioLifecycleConfigByName(conn, rs.Primary.ID)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("SageMaker Studio Lifecycle Config %s still exists", rs.Primary.ID)
	}

	return nil
}

func testAccCheckStudioLifecycleExistsConfig(n string, config *sagemaker.DescribeStudioLifecycleConfigOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No SageMaker Studio Lifecycle Config ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).SageMakerConn

		output, err := tfsagemaker.FindStudioLifecycleConfigByName(conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		*config = *output

		return nil
	}
}

func testAccStudioLifecycleBasicConfig(rName string) string {
	return fmt.Sprintf(`
resource "aws_sagemaker_studio_lifecycle_config" "test" {
  studio_lifecycle_config_name     = %[1]q
  studio_lifecycle_config_app_type = "JupyterServer"
  studio_lifecycle_config_content  = base64encode("echo Hello")
}
`, rName)
}

func testAccStudioLifecycleTags1Config(rName, tagKey1, tagValue1 string) string {
	return fmt.Sprintf(`
resource "aws_sagemaker_studio_lifecycle_config" "test" {
  studio_lifecycle_config_name     = %[1]q
  studio_lifecycle_config_app_type = "JupyterServer"
  studio_lifecycle_config_content  = base64encode("echo Hello")

  tags = {
    %[2]q = %[3]q
  }
}
`, rName, tagKey1, tagValue1)
}

func testAccStudioLifecycleTags2Config(rName, tagKey1, tagValue1, tagKey2, tagValue2 string) string {
	return fmt.Sprintf(`
resource "aws_sagemaker_studio_lifecycle_config" "test" {
  studio_lifecycle_config_name     = %[1]q
  studio_lifecycle_config_app_type = "JupyterServer"
  studio_lifecycle_config_content  = base64encode("echo Hello")

  tags = {
    %[2]q = %[3]q
    %[4]q = %[5]q
  }
}
`, rName, tagKey1, tagValue1, tagKey2, tagValue2)
}
