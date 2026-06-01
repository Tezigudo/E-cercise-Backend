package middleware

import (
	"github.com/E-cercise/E-cercise/src/enum"
	"github.com/E-cercise/E-cercise/src/helper"
	"github.com/E-cercise/E-cercise/src/model"
	"github.com/E-cercise/E-cercise/src/repository"
	"github.com/gofiber/fiber/v2"
	"strings"
)

// bearerToken returns the JWT from the "Authorization: Bearer <t>" header,
// falling back to the HttpOnly "access_token" cookie set at login. The header
// path is unchanged for API clients; the cookie path lets browsers authenticate
// without exposing the token to JavaScript.
func bearerToken(ctx *fiber.Ctx) string {
	if authHeader := ctx.Get("Authorization"); authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
		return ""
	}
	return ctx.Cookies("access_token")
}

func Authentication(userRepo repository.UserRepository) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		tokenString := bearerToken(ctx)
		if tokenString == "" {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authentication token"})
		}
		claims, err := helper.GetClaimFromToken(tokenString)

		if err != nil {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		userID, ok := claims["user_id"].(string)
		if !ok {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token payload"})
		}

		user, err := userRepo.FindByID(userID)
		if err != nil || user == nil {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}

		ctx.Locals("currentUser", user)

		return ctx.Next()
	}
}

func RoleAuthorization(allowedRoles ...enum.Role) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		currentUser := ctx.Locals("currentUser")
		if currentUser == nil {
			return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Roles not found in context"})
		}

		user, ok := currentUser.(*model.User)
		if !ok {
			return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid user context"})
		}

		if helper.ContainsRole(allowedRoles, user.Role) {
			return ctx.Next()
		}

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}
}

func OptionalAuthentication(userRepo repository.UserRepository) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		// Token from the Authorization header or the access_token cookie.
		tokenString := bearerToken(ctx)
		if tokenString == "" {
			// No token provided — continue unauthenticated.
			return ctx.Next()
		}

		// Attempt to parse the token
		claims, err := helper.GetClaimFromToken(tokenString)
		if err != nil {
			// Token is invalid, but we do not throw an error
			return ctx.Next()
		}

		// 4) Extract user info from the token
		userID, ok := claims["user_id"].(string)
		if !ok {
			// If userID is missing or incorrectly typed,
			// we skip setting user and proceed
			return ctx.Next()
		}

		// 5) Fetch the user from the DB
		user, err := userRepo.FindByID(userID)
		if err != nil || user == nil {
			// If user does not exist, skip
			return ctx.Next()
		}

		// 6) If everything is valid, store user in context
		ctx.Locals("currentUser", user)
		return ctx.Next()
	}
}
