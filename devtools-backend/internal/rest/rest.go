package rest

import "github.com/gofiber/fiber/v2"

func NewServer(port string) {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	app.Listen(":" + port)
}
