package answer

import (
	"github.com/ggmolly/belfast/internal/answer/fleettech"
	"github.com/ggmolly/belfast/internal/connection"
)

const (
	fleetTechResultFailure = fleettech.ResultFailure
	fleetTechResultSuccess = fleettech.ResultSuccess

	fleetTechOneStepClaimType = fleettech.OneStepClaimType
)

func TechnologyNationProxy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.TechnologyNationProxy(buffer, client)
}

func StartCampTech(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.StartCampTech(buffer, client)
}

func FinishCampTechnology(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.FinishCampTechnology(buffer, client)
}

func ClaimFleetTechCampAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.ClaimFleetTechCampAward(buffer, client, applyNewServerShopDropsTx)
}

func ClaimTechnologyCampAwardsOneStep(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.ClaimTechnologyCampAwardsOneStep(buffer, client, applyNewServerShopDropsTx)
}

func SetFleetTechAttrAddition(buffer *[]byte, client *connection.Client) (int, int, error) {
	return fleettech.SetFleetTechAttrAddition(buffer, client)
}
