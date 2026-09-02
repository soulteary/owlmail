package api

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	openapicontract "github.com/soulteary/owlmail/openapi"
)

func (api *API) getOpenAPIJSON(c fiber.Ctx) error {
	document, err := openapicontract.JSON(api.route("/api/v1"))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "OpenAPI contract unavailable"))
	}
	c.Set(fiber.HeaderContentType, "application/vnd.oai.openapi+json;version=3.1")
	return c.Send(document)
}

func (api *API) getOpenAPIYAML(c fiber.Ctx) error {
	document, err := openapicontract.YAML(api.route("/api/v1"))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "OpenAPI contract unavailable"))
	}
	c.Set(fiber.HeaderContentType, "application/vnd.oai.openapi;version=3.1")
	return c.Send(document)
}
