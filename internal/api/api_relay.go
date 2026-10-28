package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/relay"
)

func (api *API) relayEmail(c fiber.Ctx) error {
	relayTo := c.Query("relayTo")
	if relayTo == "" {
		var body struct {
			RelayTo string `json:"relayTo"`
		}
		if c.Bind().Body(&body) == nil {
			relayTo = body.RelayTo
		}
	}
	return api.enqueueLegacyRelay(c, relayTo)
}
func (api *API) relayEmailWithParam(c fiber.Ctx) error {
	return api.enqueueLegacyRelay(c, c.Params("relayTo"))
}
func (api *API) enqueueLegacyRelay(c fiber.Ctx, relayTo string) error {
	job, err := api.relayService.Submit(relay.Request{EmailID: c.Params("id"), RelayTo: relayTo, Origin: "legacy"})
	if err != nil {
		return relaySubmitError(c, err)
	}
	c.Set(fiber.HeaderLocation, api.route("/api/v1/relay-jobs/"+job.ID))
	return c.JSON(SuccessResponse(SuccessCodeEmailRelayed, "Email relayed successfully", fiber.Map{"relayTo": relayTo}))
}
