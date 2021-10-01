package cloudformation_test

import (
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/go-multierror"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfcloudformation "github.com/hashicorp/terraform-provider-aws/internal/service/cloudformation"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
)

func init() {
	resource.AddTestSweepers("aws_cloudformation_stack_set", &resource.Sweeper{
		Name: "aws_cloudformation_stack_set",
		Dependencies: []string{
			"aws_cloudformation_stack_set_instance",
		},
		F: sweepStackSets,
	})
}

func sweepStackSets(region string) error {
	client, err := sweep.SharedRegionalSweepClient(region)

	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	conn := client.(*conns.AWSClient).CloudFormationConn
	stackSets, err := tfcloudformation.ListStackSets(conn)

	if sweep.SkipSweepError(err) || tfawserr.ErrMessageContains(err, "ValidationError", "AWS CloudFormation StackSets is not supported") {
		log.Printf("[WARN] Skipping CloudFormation StackSet sweep for %s: %s", region, err)
		return nil
	}

	if err != nil {
		return fmt.Errorf("error listing CloudFormation StackSets: %w", err)
	}

	var sweeperErrs *multierror.Error

	for _, stackSet := range stackSets {
		input := &cloudformation.DeleteStackSetInput{
			StackSetName: stackSet.StackSetName,
		}
		name := aws.StringValue(stackSet.StackSetName)

		log.Printf("[INFO] Deleting CloudFormation StackSet: %s", name)
		_, err := conn.DeleteStackSet(input)

		if tfawserr.ErrMessageContains(err, cloudformation.ErrCodeStackSetNotFoundException, "") {
			continue
		}

		if err != nil {
			sweeperErr := fmt.Errorf("error deleting CloudFormation StackSet (%s): %w", name, err)
			log.Printf("[ERROR] %s", sweeperErr)
			sweeperErrs = multierror.Append(sweeperErrs, sweeperErr)
			continue
		}
	}

	return sweeperErrs.ErrorOrNil()
}

func TestAccCloudFormationStackSet_basic(t *testing.T) {
	var stackSet1 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	iamRoleResourceName := "aws_iam_role.test"
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetNameConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttrPair(resourceName, "administration_role_arn", iamRoleResourceName, "arn"),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "cloudformation", regexp.MustCompile(`stackset/.+`)),
					resource.TestCheckResourceAttr(resourceName, "capabilities.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "description", ""),
					resource.TestCheckResourceAttr(resourceName, "execution_role_name", "AWSCloudFormationStackSetExecutionRole"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "permission_model", "SELF_MANAGED"),
					resource.TestMatchResourceAttr(resourceName, "stack_set_id", regexp.MustCompile(fmt.Sprintf("%s:.+", rName))),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "template_body", testAccStackSetTemplateBodyVPC(rName)+"\n"),
					resource.TestCheckNoResourceAttr(resourceName, "template_url"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
		},
	})
}

func TestAccCloudFormationStackSet_disappears(t *testing.T) {
	var stackSet1 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetNameConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					acctest.CheckResourceDisappears(acctest.Provider, tfcloudformation.ResourceStackSet(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccCloudFormationStackSet_administrationRoleARN(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	iamRoleResourceName1 := "aws_iam_role.test1"
	iamRoleResourceName2 := "aws_iam_role.test2"
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetAdministrationRoleARN1Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttrPair(resourceName, "administration_role_arn", iamRoleResourceName1, "arn"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetAdministrationRoleARN2Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttrPair(resourceName, "administration_role_arn", iamRoleResourceName2, "arn"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_description(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetDescriptionConfig(rName, "description1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "description", "description1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetDescriptionConfig(rName, "description2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "description", "description2"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_executionRoleName(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetExecutionRoleNameConfig(rName, "name1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "execution_role_name", "name1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetExecutionRoleNameConfig(rName, "name2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "execution_role_name", "name2"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_name(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName1 := sdkacctest.RandomWithPrefix("tf-acc-test")
	rName2 := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccStackSetNameConfig(""),
				ExpectError: regexp.MustCompile(`expected length`),
			},
			{
				Config:      testAccStackSetNameConfig(sdkacctest.RandStringFromCharSet(129, sdkacctest.CharSetAlpha)),
				ExpectError: regexp.MustCompile(`(cannot be longer|expected length)`),
			},
			{
				Config:      testAccStackSetNameConfig("1"),
				ExpectError: regexp.MustCompile(`must begin with alphabetic character`),
			},
			{
				Config:      testAccStackSetNameConfig("a_b"),
				ExpectError: regexp.MustCompile(`must contain only alphanumeric and hyphen characters`),
			},
			{
				Config: testAccStackSetNameConfig(rName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "name", rName1),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetNameConfig(rName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "name", rName2),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_parameters(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetParameters1Config(rName, "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetParameters2Config(rName, "value1updated", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter2", "value2"),
				),
			},
			{
				Config: testAccStackSetParameters1Config(rName, "value1updated"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "value1updated"),
				),
			},
			{
				Config: testAccStackSetNameConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "0"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_Parameters_default(t *testing.T) {
	acctest.Skip(t, "this resource does not currently ignore unconfigured CloudFormation template parameters with the Default property")
	// Additional references:
	//  * https://github.com/hashicorp/terraform/issues/18863

	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetParametersDefault0Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "defaultvalue"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetParametersDefault1Config(rName, "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "value1"),
				),
			},
			{
				Config: testAccStackSetParametersDefault0Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "defaultvalue"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_Parameters_noEcho(t *testing.T) {
	acctest.Skip(t, "this resource does not currently ignore CloudFormation template parameters with the NoEcho property")
	// Additional references:
	//  * https://github.com/hashicorp/terraform-provider-aws/issues/55

	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetParametersNoEcho1Config(rName, "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "****"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetParametersNoEcho1Config(rName, "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "parameters.Parameter1", "****"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_PermissionModel_serviceManaged(t *testing.T) {
	acctest.Skip(t, "API does not support enabling Organizations access (in particular, creating the Stack Sets IAM Service-Linked Role)")

	var stackSet1 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			testAccPreCheckStackSet(t)
			acctest.PreCheckOrganizationsAccount(t)
		},
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID, "organizations"),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetPermissionModelConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "cloudformation", regexp.MustCompile(`stackset/.+`)),
					resource.TestCheckResourceAttr(resourceName, "permission_model", "SERVICE_MANAGED"),
					resource.TestCheckResourceAttr(resourceName, "auto_deployment.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "auto_deployment.0.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "auto_deployment.0.retain_stacks_on_account_removal", "false"),
					resource.TestMatchResourceAttr(resourceName, "stack_set_id", regexp.MustCompile(fmt.Sprintf("%s:.+", rName))),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
		},
	})
}

func TestAccCloudFormationStackSet_tags(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetTags1Config(rName, "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.Key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetTags2Config(rName, "value1updated", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.Key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.Key2", "value2"),
				),
			},
			{
				Config: testAccStackSetTags1Config(rName, "value1updated"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.Key1", "value1updated"),
				),
			},
			{
				Config: testAccStackSetNameConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_templateBody(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetTemplateBodyConfig(rName, testAccStackSetTemplateBodyVPC(rName+"1")),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttr(resourceName, "template_body", testAccStackSetTemplateBodyVPC(rName+"1")+"\n"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetTemplateBodyConfig(rName, testAccStackSetTemplateBodyVPC(rName+"2")),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttr(resourceName, "template_body", testAccStackSetTemplateBodyVPC(rName+"2")+"\n"),
				),
			},
		},
	})
}

func TestAccCloudFormationStackSet_templateURL(t *testing.T) {
	var stackSet1, stackSet2 cloudformation.StackSet
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudformation_stack_set.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckStackSet(t) },
		ErrorCheck:   acctest.ErrorCheck(t, cloudformation.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStackSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackSetTemplateURL1Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet1),
					resource.TestCheckResourceAttrSet(resourceName, "template_body"),
					resource.TestCheckResourceAttrSet(resourceName, "template_url"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"template_url",
				},
			},
			{
				Config: testAccStackSetTemplateURL2Config(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudFormationStackSetExists(resourceName, &stackSet2),
					testAccCheckCloudFormationStackSetNotRecreated(&stackSet1, &stackSet2),
					resource.TestCheckResourceAttrSet(resourceName, "template_body"),
					resource.TestCheckResourceAttrSet(resourceName, "template_url"),
				),
			},
		},
	})
}

func testAccCheckCloudFormationStackSetExists(resourceName string, stackSet *cloudformation.StackSet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]

		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).CloudFormationConn

		input := &cloudformation.DescribeStackSetInput{
			StackSetName: aws.String(rs.Primary.ID),
		}

		output, err := conn.DescribeStackSet(input)

		if err != nil {
			return err
		}

		if output == nil || output.StackSet == nil {
			return fmt.Errorf("CloudFormation StackSet (%s) not found", rs.Primary.ID)
		}

		*stackSet = *output.StackSet

		return nil
	}
}

func testAccCheckStackSetDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).CloudFormationConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_cloudformation_stack_set" {
			continue
		}

		input := cloudformation.DescribeStackSetInput{
			StackSetName: aws.String(rs.Primary.ID),
		}

		output, err := conn.DescribeStackSet(&input)

		if tfawserr.ErrMessageContains(err, cloudformation.ErrCodeStackSetNotFoundException, "") {
			return nil
		}

		if err != nil {
			return err
		}

		if output != nil && output.StackSet != nil {
			return fmt.Errorf("CloudFormation StackSet (%s) still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckCloudFormationStackSetNotRecreated(i, j *cloudformation.StackSet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.StringValue(i.StackSetId) != aws.StringValue(j.StackSetId) {
			return fmt.Errorf("CloudFormation StackSet (%s) recreated", aws.StringValue(i.StackSetName))
		}

		return nil
	}
}

func testAccCheckCloudFormationStackSetRecreated(i, j *cloudformation.StackSet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.StringValue(i.StackSetId) == aws.StringValue(j.StackSetId) {
			return fmt.Errorf("CloudFormation StackSet (%s) not recreated", aws.StringValue(i.StackSetName))
		}

		return nil
	}
}

func testAccPreCheckStackSet(t *testing.T) {
	conn := acctest.Provider.Meta().(*conns.AWSClient).CloudFormationConn

	input := &cloudformation.ListStackSetsInput{}
	_, err := conn.ListStackSets(input)

	if acctest.PreCheckSkipError(err) || tfawserr.ErrMessageContains(err, "ValidationError", "AWS CloudFormation StackSets is not supported") {
		t.Skipf("skipping acceptance testing: %s", err)
	}

	if err != nil {
		t.Fatalf("unexpected PreCheck error: %s", err)
	}
}

func testAccStackSetTemplateBodyParameters1(rName string) string {
	return fmt.Sprintf(`
Parameters:
  Parameter1:
    Type: String
Resources:
  TestVpc:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: 10.0.0.0/16
      Tags:
        - Key: Name
          Value: %[1]q
Outputs:
  Parameter1Value:
    Value: !Ref Parameter1
  Region:
    Value: !Ref "AWS::Region"
  TestVpcID:
    Value: !Ref TestVpc
`, rName)
}

func testAccStackSetTemplateBodyParameters2(rName string) string {
	return fmt.Sprintf(`
Parameters:
  Parameter1:
    Type: String
  Parameter2:
    Type: String
Resources:
  TestVpc:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: 10.0.0.0/16
      Tags:
        - Key: Name
          Value: %[1]q
Outputs:
  Parameter1Value:
    Value: !Ref Parameter1
  Parameter2Value:
    Value: !Ref Parameter2
  Region:
    Value: !Ref "AWS::Region"
  TestVpcID:
    Value: !Ref TestVpc
`, rName)
}

func testAccStackSetTemplateBodyParametersDefault1(rName string) string {
	return fmt.Sprintf(`
Parameters:
  Parameter1:
    Type: String
    Default: defaultvalue
Resources:
  TestVpc:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: 10.0.0.0/16
      Tags:
        - Key: Name
          Value: %[1]q
Outputs:
  Parameter1Value:
    Value: !Ref Parameter1
  Region:
    Value: !Ref "AWS::Region"
  TestVpcID:
    Value: !Ref TestVpc
`, rName)
}

func testAccStackSetTemplateBodyParametersNoEcho1(rName string) string {
	return fmt.Sprintf(`
Parameters:
  Parameter1:
    Type: String
    NoEcho: true
Resources:
  TestVpc:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: 10.0.0.0/16
      Tags:
        - Key: Name
          Value: %[1]q
Outputs:
  Parameter1Value:
    Value: !Ref Parameter1
  Region:
    Value: !Ref "AWS::Region"
  TestVpcID:
    Value: !Ref TestVpc
`, rName)
}

func testAccStackSetTemplateBodyVPC(rName string) string {
	return fmt.Sprintf(`
Resources:
  TestVpc:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: 10.0.0.0/16
      Tags:
        - Key: Name
          Value: %[1]q
Outputs:
  Region:
    Value: !Ref "AWS::Region"
  TestVpcID:
    Value: !Ref TestVpc
`, rName)
}

func testAccStackSetAdministrationRoleARN1Config(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test1" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = "%[1]s1"
}

resource "aws_iam_role" "test2" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = "%[1]s2"
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test1.arn
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName))
}

func testAccStackSetAdministrationRoleARN2Config(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test1" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = "%[1]s1"
}

resource "aws_iam_role" "test2" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = "%[1]s2"
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test2.arn
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName))
}

func testAccStackSetDescriptionConfig(rName, description string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  description             = %[3]q
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName), description)
}

func testAccStackSetExecutionRoleNameConfig(rName, executionRoleName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  execution_role_name     = %[3]q
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName), executionRoleName)
}

func testAccStackSetNameConfig(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName))
}

func testAccStackSetParameters1Config(rName, value1 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  parameters = {
    Parameter1 = %[3]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyParameters1(rName), value1)
}

func testAccStackSetParameters2Config(rName, value1, value2 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  parameters = {
    Parameter1 = %[3]q
    Parameter2 = %[4]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyParameters2(rName), value1, value2)
}

func testAccStackSetParametersDefault0Config(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyParametersDefault1(rName))
}

func testAccStackSetParametersDefault1Config(rName, value1 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  parameters = {
    Parameter1 = %[3]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyParametersDefault1(rName), value1)
}

func testAccStackSetParametersNoEcho1Config(rName, value1 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  parameters = {
    Parameter1 = %[3]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyParametersNoEcho1(rName), value1)
}

func testAccStackSetTags1Config(rName, value1 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  tags = {
    Key1 = %[3]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName), value1)
}

func testAccStackSetTags2Config(rName, value1, value2 string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  tags = {
    Key1 = %[3]q
    Key2 = %[4]q
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName), value1, value2)
}

func testAccStackSetTemplateBodyConfig(rName, templateBody string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, templateBody)
}

func testAccStackSetTemplateURL1Config(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_s3_bucket" "test" {
  acl    = "public-read"
  bucket = %[1]q
}

resource "aws_s3_bucket_object" "test" {
  acl    = "public-read"
  bucket = aws_s3_bucket.test.bucket

  content = <<CONTENT
%[2]s
CONTENT

  key = "%[1]s-template1.yml"
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q
  template_url            = "https://${aws_s3_bucket.test.bucket_regional_domain_name}/${aws_s3_bucket_object.test.key}"
}
`, rName, testAccStackSetTemplateBodyVPC(rName+"1"))
}

func testAccStackSetTemplateURL2Config(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "cloudformation.amazonaws.com"
        ]
      },
      "Action": [
        "sts:AssumeRole"
      ]
    }
  ]
}
EOF

  name = %[1]q
}

resource "aws_s3_bucket" "test" {
  acl    = "public-read"
  bucket = %[1]q
}

resource "aws_s3_bucket_object" "test" {
  acl    = "public-read"
  bucket = aws_s3_bucket.test.bucket

  content = <<CONTENT
%[2]s
CONTENT

  key = "%[1]s-template2.yml"
}

resource "aws_cloudformation_stack_set" "test" {
  administration_role_arn = aws_iam_role.test.arn
  name                    = %[1]q
  template_url            = "https://${aws_s3_bucket.test.bucket_regional_domain_name}/${aws_s3_bucket_object.test.key}"
}
`, rName, testAccStackSetTemplateBodyVPC(rName+"2"))
}

func testAccStackSetPermissionModelConfig(rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudformation_stack_set" "test" {
  name             = %[1]q
  permission_model = "SERVICE_MANAGED"

  auto_deployment {
    enabled                          = true
    retain_stacks_on_account_removal = false
  }

  template_body = <<TEMPLATE
%[2]s
TEMPLATE
}
`, rName, testAccStackSetTemplateBodyVPC(rName))
}
