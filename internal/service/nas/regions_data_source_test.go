package nas_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
)

func TestAccNASRegionsDataSource_basic(t *testing.T) {
	resourceName := "data.aws_regions.empty"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:   func() { acctest.PreCheck(t) },
		ErrorCheck: acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:  acctest.Providers,
		Steps: []resource.TestStep{
			{
				Config: testAccRegionsDataSourceConfig_empty(),
				Check: resource.ComposeTestCheckFunc(
					testAccRegionsCheckDataSource(resourceName),
					resource.TestCheckNoResourceAttr(resourceName, "all_regions"),
				),
			},
		},
	})
}

func TestAccNASRegionsDataSource_filter(t *testing.T) {
	resourceName := "data.aws_regions.opt_in_status"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:   func() { acctest.PreCheck(t) },
		ErrorCheck: acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:  acctest.Providers,
		Steps: []resource.TestStep{
			{
				Config: testAccRegionsDataSourceConfig_allRegionsFiltered("opt-in-not-required"),
				Check: resource.ComposeTestCheckFunc(
					testAccRegionsCheckDataSource(resourceName),
				),
			},
		},
	})
}

func TestAccNASRegionsDataSource_allRegions(t *testing.T) {
	resourceAllRegions := "data.aws_regions.all_regions"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:   func() { acctest.PreCheck(t) },
		ErrorCheck: acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:  acctest.Providers,
		Steps: []resource.TestStep{
			{
				Config: testAccRegionsDataSourceConfig_allRegions(),
				Check: resource.ComposeTestCheckFunc(
					testAccRegionsCheckDataSource(resourceAllRegions),
					resource.TestCheckResourceAttr(resourceAllRegions, "all_regions", "true"),
					resource.TestCheckNoResourceAttr(resourceAllRegions, "opt_in_status"),
				),
			},
		},
	})
}

func testAccRegionsCheckDataSource(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]

		if !ok {
			return fmt.Errorf("root module has no resource called %s", resourceName)
		}

		names, namesOk := rs.Primary.Attributes["names.#"]

		if !namesOk {
			return fmt.Errorf("names attribute is missing.")
		}

		namesQuantity, err := strconv.Atoi(names)

		if err != nil {
			return fmt.Errorf("error parsing names (%s) into integer: %s", names, err)
		}

		if namesQuantity == 0 {
			return fmt.Errorf("No names found, this is probably a bug.")
		}

		return nil
	}
}

func testAccRegionsDataSourceConfig_empty() string {
	return `
data "aws_regions" "empty" {}
`
}

func testAccRegionsDataSourceConfig_allRegions() string {
	return `
data "aws_regions" "all_regions" {
  all_regions = "true"
}
`
}

func testAccRegionsDataSourceConfig_allRegionsFiltered(filter string) string {
	return fmt.Sprintf(`
data "aws_regions" "opt_in_status" {
  filter {
    name   = "opt-in-status"
    values = ["%s"]
  }
}
`, filter)
}
