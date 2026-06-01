package controller

import (
	"github.com/E-cercise/E-cercise/src/config"
	"github.com/E-cercise/E-cercise/src/data/request"
	"github.com/E-cercise/E-cercise/src/service"
	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	UserService service.UserService
}

func NewAuthControllerImpl(userService service.UserService) *AuthController {
	return &AuthController{
		UserService: userService,
	}
}

func (c *AuthController) UserRegister(ctx *fiber.Ctx) error {
	reqBody := ctx.Locals("reqBody").(request.RegisterRequest)

	err := c.UserService.RegisterUser(reqBody)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User registered successfully",
	})
}

func (c *AuthController) Login(ctx *fiber.Ctx) error {
	loginBody, ok := ctx.Locals("loginBody").(request.LoginRequest)
	if !ok {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	accessToken, err := c.UserService.LoginUser(loginBody)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credentials",
		})
	}

	// Also set the token as an HttpOnly cookie so browsers can authenticate
	// without exposing it to JS (additive — the Bearer-header flow is unchanged).
	// NOTE: a cross-origin SPA (FE :5173, BE :8888) needs SameSite=None + Secure
	// over HTTPS for the browser to send this; behind a same-origin reverse proxy
	// (or same-site dev) Lax suffices. See the PR for the FE-migration decisions.
	ctx.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    *accessToken,
		HTTPOnly: true,
		Secure:   config.CookieSecure,   // COOKIE_SECURE=true in HTTPS prod
		SameSite: config.CookieSameSite, // COOKIE_SAMESITE=None for a cross-site SPA
		Path:     "/",
		MaxAge:   3 * 60 * 60, // 3h, matches the JWT exp
	})

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"access_token": accessToken,
	})
}
