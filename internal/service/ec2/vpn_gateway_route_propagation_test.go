package ec2_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfec2 "github.com/hashicorp/terraform-provider-aws/internal/service/ec2"
)

func TestAccEC2VPNGatewayRoutePropagation_basic(t *testing.T) {
	var rtID, gwID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:   func() { acctest.PreCheck(t) },
		ErrorCheck: acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:  acctest.Providers,
		Steps: []resource.TestStep{
			{
				Config: testAccVPNGatewayRoutePropagation_basic,
				Check: func(state *terraform.State) error {
					conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Conn

					rs := state.RootModule().Resources["aws_vpn_gateway_route_propagation.foo"]
					if rs == nil {
						return errors.New("missing resource state")
					}

					rtID = rs.Primary.Attributes["route_table_id"]
					gwID = rs.Primary.Attributes["vpn_gateway_id"]

					rt, err := tfec2.WaitRouteTableReady(conn, rtID)

					if err != nil {
						return fmt.Errorf("error getting route table (%s) while checking VPN gateway route propagation: %w", rtID, err)
					}

					if rt == nil {
						return errors.New("route table doesn't exist")
					}

					exists := false
					for _, vgw := range rt.PropagatingVgws {
						if *vgw.GatewayId == gwID {
							exists = true
						}
					}
					if !exists {
						return errors.New("route table does not list VPN gateway as a propagator")
					}

					return nil
				},
			},
		},
		CheckDestroy: func(state *terraform.State) error {
			conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Conn

			rt, err := tfec2.WaitRouteTableDeleted(conn, rtID)

			if err != nil {
				return fmt.Errorf("error getting route table (%s) status while checking destroy: %w", rtID, err)
			}

			if rt != nil {
				return errors.New("route table still exists")
			}
			return nil
		},
	})

}

const testAccVPNGatewayRoutePropagation_basic = `
resource "aws_vpc" "foo" {
  cidr_block = "10.1.0.0/16"

  tags = {
    Name = "terraform-testacc-vpn-gateway-route-propagation"
  }
}

resource "aws_vpn_gateway" "foo" {
  vpc_id = aws_vpc.foo.id
}

resource "aws_route_table" "foo" {
  vpc_id = aws_vpc.foo.id
}

resource "aws_vpn_gateway_route_propagation" "foo" {
  vpn_gateway_id = aws_vpn_gateway.foo.id
  route_table_id = aws_route_table.foo.id
}
`
