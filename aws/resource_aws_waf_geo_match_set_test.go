package aws

import (
	"fmt"
	"log"
	"regexp"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/waf"
	"github.com/hashicorp/go-multierror"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/aws/internal/service/waf/lister"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
)

func init() {
	resource.AddTestSweepers("aws_waf_geo_match_set", &resource.Sweeper{
		Name: "aws_waf_geo_match_set",
		F:    testSweepWafGeoMatchSet,
		Dependencies: []string{
			"aws_waf_rate_based_rule",
			"aws_waf_rule",
			"aws_waf_rule_group",
		},
	})
}

func testSweepWafGeoMatchSet(region string) error {
	client, err := sweep.SharedRegionalSweepClient(region)

	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}

	conn := client.(*conns.AWSClient).WAFConn
	sweepResources := make([]*sweep.SweepResource, 0)
	var errs *multierror.Error
	var g multierror.Group
	var mutex = &sync.Mutex{}

	input := &waf.ListGeoMatchSetsInput{}

	err = lister.ListGeoMatchSetsPages(conn, input, func(page *waf.ListGeoMatchSetsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, geoMatchSet := range page.GeoMatchSets {
			r := ResourceGeoMatchSet()
			d := r.Data(nil)

			id := aws.StringValue(geoMatchSet.GeoMatchSetId)
			d.SetId(id)

			// read concurrently and gather errors
			g.Go(func() error {
				// Need to Read first to fill in geo_match_constraint attribute
				err := r.Read(d, client)

				if err != nil {
					sweeperErr := fmt.Errorf("error reading WAF Geo Match Set (%s): %w", id, err)
					log.Printf("[ERROR] %s", sweeperErr)
					return sweeperErr
				}

				// In case it was already deleted
				if d.Id() == "" {
					return nil
				}

				mutex.Lock()
				defer mutex.Unlock()
				sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))

				return nil
			})
		}

		return !lastPage
	})

	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error listing WAF Geo Match Set for %s: %w", region, err))
	}

	if err = g.Wait().ErrorOrNil(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error concurrently reading WAF Geo Match Sets: %w", err))
	}

	if err = sweep.SweepOrchestrator(sweepResources); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error sweeping WAF Geo Match Set for %s: %w", region, err))
	}

	if sweep.SkipSweepError(errs.ErrorOrNil()) {
		log.Printf("[WARN] Skipping WAF Geo Match Set sweep for %s: %s", region, errs)
		return nil
	}

	return errs.ErrorOrNil()
}

func TestAccAWSWafGeoMatchSet_basic(t *testing.T) {
	var v waf.GeoMatchSet
	geoMatchSet := fmt.Sprintf("geoMatchSet-%s", sdkacctest.RandString(5))
	resourceName := "aws_waf_geo_match_set.geo_match_set"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSWaf(t) },
		ErrorCheck:   acctest.ErrorCheck(t, waf.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafGeoMatchSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSWafGeoMatchSetConfig(geoMatchSet),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &v),
					acctest.MatchResourceAttrGlobalARN(resourceName, "arn", "waf", regexp.MustCompile(`geomatchset/.+`)),
					resource.TestCheckResourceAttr(resourceName, "name", geoMatchSet),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "US",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "CA",
					}),
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

func TestAccAWSWafGeoMatchSet_changeNameForceNew(t *testing.T) {
	var before, after waf.GeoMatchSet
	geoMatchSet := fmt.Sprintf("geoMatchSet-%s", sdkacctest.RandString(5))
	geoMatchSetNewName := fmt.Sprintf("geoMatchSetNewName-%s", sdkacctest.RandString(5))
	resourceName := "aws_waf_geo_match_set.geo_match_set"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSWaf(t) },
		ErrorCheck:   acctest.ErrorCheck(t, waf.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafGeoMatchSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSWafGeoMatchSetConfig(geoMatchSet),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &before),
					resource.TestCheckResourceAttr(resourceName, "name", geoMatchSet),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "2"),
				),
			},
			{
				Config: testAccAWSWafGeoMatchSetConfigChangeName(geoMatchSetNewName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &after),
					resource.TestCheckResourceAttr(resourceName, "name", geoMatchSetNewName),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "2"),
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

func TestAccAWSWafGeoMatchSet_disappears(t *testing.T) {
	var v waf.GeoMatchSet
	geoMatchSet := fmt.Sprintf("geoMatchSet-%s", sdkacctest.RandString(5))
	resourceName := "aws_waf_geo_match_set.geo_match_set"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSWaf(t) },
		ErrorCheck:   acctest.ErrorCheck(t, waf.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafGeoMatchSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSWafGeoMatchSetConfig(geoMatchSet),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &v),
					testAccCheckAWSWafGeoMatchSetDisappears(&v),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSWafGeoMatchSet_changeConstraints(t *testing.T) {
	var before, after waf.GeoMatchSet
	setName := fmt.Sprintf("geoMatchSet-%s", sdkacctest.RandString(5))
	resourceName := "aws_waf_geo_match_set.geo_match_set"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSWaf(t) },
		ErrorCheck:   acctest.ErrorCheck(t, waf.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafGeoMatchSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSWafGeoMatchSetConfig(setName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &before),
					resource.TestCheckResourceAttr(resourceName, "name", setName),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "US",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "CA",
					}),
				),
			},
			{
				Config: testAccAWSWafGeoMatchSetConfig_changeConstraints(setName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &after),
					resource.TestCheckResourceAttr(resourceName, "name", setName),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "RU",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "geo_match_constraint.*", map[string]string{
						"type":  "Country",
						"value": "CN",
					}),
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

func TestAccAWSWafGeoMatchSet_noConstraints(t *testing.T) {
	var ipset waf.GeoMatchSet
	setName := fmt.Sprintf("geoMatchSet-%s", sdkacctest.RandString(5))
	resourceName := "aws_waf_geo_match_set.geo_match_set"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSWaf(t) },
		ErrorCheck:   acctest.ErrorCheck(t, waf.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafGeoMatchSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSWafGeoMatchSetConfig_noConstraints(setName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSWafGeoMatchSetExists(resourceName, &ipset),
					resource.TestCheckResourceAttr(resourceName, "name", setName),
					resource.TestCheckResourceAttr(resourceName, "geo_match_constraint.#", "0"),
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

func testAccCheckAWSWafGeoMatchSetDisappears(v *waf.GeoMatchSet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.Provider.Meta().(*conns.AWSClient).WAFConn

		wr := newWafRetryer(conn)
		_, err := wr.RetryWithToken(func(token *string) (interface{}, error) {
			req := &waf.UpdateGeoMatchSetInput{
				ChangeToken:   token,
				GeoMatchSetId: v.GeoMatchSetId,
			}

			for _, geoMatchConstraint := range v.GeoMatchConstraints {
				geoMatchConstraintUpdate := &waf.GeoMatchSetUpdate{
					Action: aws.String("DELETE"),
					GeoMatchConstraint: &waf.GeoMatchConstraint{
						Type:  geoMatchConstraint.Type,
						Value: geoMatchConstraint.Value,
					},
				}
				req.Updates = append(req.Updates, geoMatchConstraintUpdate)
			}
			return conn.UpdateGeoMatchSet(req)
		})
		if err != nil {
			return fmt.Errorf("Error updating GeoMatchSet: %s", err)
		}

		_, err = wr.RetryWithToken(func(token *string) (interface{}, error) {
			opts := &waf.DeleteGeoMatchSetInput{
				ChangeToken:   token,
				GeoMatchSetId: v.GeoMatchSetId,
			}
			return conn.DeleteGeoMatchSet(opts)
		})
		if err != nil {
			return fmt.Errorf("Error deleting GeoMatchSet: %s", err)
		}
		return nil
	}
}

func testAccCheckAWSWafGeoMatchSetExists(n string, v *waf.GeoMatchSet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No WAF GeoMatchSet ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).WAFConn
		resp, err := conn.GetGeoMatchSet(&waf.GetGeoMatchSetInput{
			GeoMatchSetId: aws.String(rs.Primary.ID),
		})

		if err != nil {
			return err
		}

		if *resp.GeoMatchSet.GeoMatchSetId == rs.Primary.ID {
			*v = *resp.GeoMatchSet
			return nil
		}

		return fmt.Errorf("WAF GeoMatchSet (%s) not found", rs.Primary.ID)
	}
}

func testAccCheckAWSWafGeoMatchSetDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_waf_geo_match_set" {
			continue
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).WAFConn
		resp, err := conn.GetGeoMatchSet(
			&waf.GetGeoMatchSetInput{
				GeoMatchSetId: aws.String(rs.Primary.ID),
			})

		if err == nil {
			if *resp.GeoMatchSet.GeoMatchSetId == rs.Primary.ID {
				return fmt.Errorf("WAF GeoMatchSet %s still exists", rs.Primary.ID)
			}
		}

		// Return nil if the GeoMatchSet is already destroyed
		if tfawserr.ErrMessageContains(err, waf.ErrCodeNonexistentItemException, "") {
			return nil
		}

		return err
	}

	return nil
}

func testAccAWSWafGeoMatchSetConfig(name string) string {
	return fmt.Sprintf(`
resource "aws_waf_geo_match_set" "geo_match_set" {
  name = "%s"

  geo_match_constraint {
    type  = "Country"
    value = "US"
  }

  geo_match_constraint {
    type  = "Country"
    value = "CA"
  }
}
`, name)
}

func testAccAWSWafGeoMatchSetConfigChangeName(name string) string {
	return fmt.Sprintf(`
resource "aws_waf_geo_match_set" "geo_match_set" {
  name = "%s"

  geo_match_constraint {
    type  = "Country"
    value = "US"
  }

  geo_match_constraint {
    type  = "Country"
    value = "CA"
  }
}
`, name)
}

func testAccAWSWafGeoMatchSetConfig_changeConstraints(name string) string {
	return fmt.Sprintf(`
resource "aws_waf_geo_match_set" "geo_match_set" {
  name = "%s"

  geo_match_constraint {
    type  = "Country"
    value = "RU"
  }

  geo_match_constraint {
    type  = "Country"
    value = "CN"
  }
}
`, name)
}

func testAccAWSWafGeoMatchSetConfig_noConstraints(name string) string {
	return fmt.Sprintf(`
resource "aws_waf_geo_match_set" "geo_match_set" {
  name = "%s"
}
`, name)
}
