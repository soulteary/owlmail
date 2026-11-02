package api

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	openapicontract "github.com/soulteary/owlmail/openapi"
)

func (api *API) getOpenAPIJSON(c fiber.Ctx) error {
	var document []byte
	var err error
	if api.multiuser != nil {
		subject, email, verified := api.multiuser.Proxy.IdentityHeaders()
		document, err = openapicontract.MultiuserJSONWithHeaders(api.route("/api/v1"), subject, email, verified)
	} else {
		document, err = openapicontract.JSON(api.route("/api/v1"))
	}
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "OpenAPI contract unavailable"))
	}
	c.Set(fiber.HeaderContentType, "application/vnd.oai.openapi+json;version=3.1")
	return c.Send(document)
}

func (api *API) getOpenAPIYAML(c fiber.Ctx) error {
	var document []byte
	var err error
	if api.multiuser != nil {
		subject, email, verified := api.multiuser.Proxy.IdentityHeaders()
		document, err = openapicontract.MultiuserYAMLWithHeaders(api.route("/api/v1"), subject, email, verified)
	} else {
		document, err = openapicontract.YAML(api.route("/api/v1"))
	}
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "OpenAPI contract unavailable"))
	}
	c.Set(fiber.HeaderContentType, "application/vnd.oai.openapi;version=3.1")
	return c.Send(document)
}
