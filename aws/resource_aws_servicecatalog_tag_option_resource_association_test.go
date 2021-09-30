package aws

import (
	"fmt"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/servicecatalog"
	multierror "github.com/hashicorp/go-multierror"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	tfservicecatalog "github.com/hashicorp/terraform-provider-aws/aws/internal/service/servicecatalog"
	"github.com/hashicorp/terraform-provider-aws/aws/internal/service/servicecatalog/waiter"
	"github.com/hashicorp/terraform-provider-aws/aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
)

// add sweeper to delete known test servicecat tag option resource associations
func init() {
	resource.AddTestSweepers("aws_servicecatalog_tag_option_resource_association", &resource.Sweeper{
		Name:         "aws_servicecatalog_tag_option_resource_association",
		Dependencies: []string{},
		F:            testSweepServiceCatalogTagOptionResourceAssociations,
	})
}

func testSweepServiceCatalogTagOptionResourceAssociations(region string) error {
	client, err := sweep.SharedRegionalSweepClient(region)

	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}

	conn := client.(*conns.AWSClient).ServiceCatalogConn
	sweepResources := make([]*sweep.SweepResource, 0)
	var errs *multierror.Error

	input := &servicecatalog.ListTagOptionsInput{}

	err = conn.ListTagOptionsPages(input, func(page *servicecatalog.ListTagOptionsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, tag := range page.TagOptionDetails {
			if tag == nil {
				continue
			}

			resInput := &servicecatalog.ListResourcesForTagOptionInput{
				TagOptionId: tag.Id,
			}

			err = conn.ListResourcesForTagOptionPages(resInput, func(page *servicecatalog.ListResourcesForTagOptionOutput, lastPage bool) bool {
				if page == nil {
					return !lastPage
				}

				for _, resource := range page.ResourceDetails {
					if resource == nil {
						continue
					}

					r := ResourceTagOptionResourceAssociation()
					d := r.Data(nil)
					d.SetId(aws.StringValue(resource.Id))

					sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))
				}

				return !lastPage
			})
		}

		return !lastPage
	})

	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error describing Service Catalog Tag Option Resource Associations for %s: %w", region, err))
	}

	if err = sweep.SweepOrchestrator(sweepResources); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error sweeping Service Catalog Tag Option Resource Associations for %s: %w", region, err))
	}

	if sweep.SkipSweepError(errs.ErrorOrNil()) {
		log.Printf("[WARN] Skipping Service Catalog Tag Option Resource Associations sweep for %s: %s", region, errs)
		return nil
	}

	return errs.ErrorOrNil()
}

func TestAccAWSServiceCatalogTagOptionResourceAssociation_basic(t *testing.T) {
	resourceName := "aws_servicecatalog_tag_option_resource_association.test"
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, servicecatalog.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAwsServiceCatalogTagOptionResourceAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSServiceCatalogTagOptionResourceAssociationConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAwsServiceCatalogTagOptionResourceAssociationExists(resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "resource_id", "aws_servicecatalog_portfolio.test", "id"),
					resource.TestCheckResourceAttrPair(resourceName, "tag_option_id", "aws_servicecatalog_tag_option.test", "id"),
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

func TestAccAWSServiceCatalogTagOptionResourceAssociation_disappears(t *testing.T) {
	resourceName := "aws_servicecatalog_tag_option_resource_association.test"
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, servicecatalog.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAwsServiceCatalogTagOptionResourceAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSServiceCatalogTagOptionResourceAssociationConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAwsServiceCatalogTagOptionResourceAssociationExists(resourceName),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceTagOptionResourceAssociation(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckAwsServiceCatalogTagOptionResourceAssociationDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).ServiceCatalogConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_servicecatalog_tag_option_resource_association" {
			continue
		}

		tagOptionID, resourceID, err := tfservicecatalog.TagOptionResourceAssociationParseID(rs.Primary.ID)

		if err != nil {
			return fmt.Errorf("could not parse ID (%s): %w", rs.Primary.ID, err)
		}

		err = waiter.TagOptionResourceAssociationDeleted(conn, tagOptionID, resourceID)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return fmt.Errorf("waiting for Service Catalog Tag Option Resource Association to be destroyed (%s): %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckAwsServiceCatalogTagOptionResourceAssociationExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]

		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		tagOptionID, resourceID, err := tfservicecatalog.TagOptionResourceAssociationParseID(rs.Primary.ID)

		if err != nil {
			return fmt.Errorf("could not parse ID (%s): %w", rs.Primary.ID, err)
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).ServiceCatalogConn

		_, err = waiter.TagOptionResourceAssociationReady(conn, tagOptionID, resourceID)

		if err != nil {
			return fmt.Errorf("waiting for Service Catalog Tag Option Resource Association existence (%s): %w", rs.Primary.ID, err)
		}

		return nil
	}
}

func testAccAWSServiceCatalogTagOptionResourceAssociationConfig_base(rName string) string {
	return fmt.Sprintf(`
resource "aws_servicecatalog_portfolio" "test" {
  name          = %[1]q
  description   = %[1]q
  provider_name = %[1]q
}

resource "aws_servicecatalog_tag_option" "test" {
  key   = %[1]q
  value = %[1]q
}
`, rName)
}

func testAccAWSServiceCatalogTagOptionResourceAssociationConfig_basic(rName string) string {
	return acctest.ConfigCompose(testAccAWSServiceCatalogTagOptionResourceAssociationConfig_base(rName), `
resource "aws_servicecatalog_tag_option_resource_association" "test" {
  resource_id   = aws_servicecatalog_portfolio.test.id
  tag_option_id = aws_servicecatalog_tag_option.test.id
}
`)
}
