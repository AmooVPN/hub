package app

import (
	"fmt"
	"html"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func renderErrorPage(title, message, requestID string) string {
	body := `<div class="container py-5"><div class="row justify-content-center"><div class="col-12 col-md-8 col-lg-6"><div class="card shadow-sm"><div class="card-body"><h1 class="h4 mb-2">` + html.EscapeString(title) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(message) + `</p>`
	if strings.TrimSpace(requestID) != "" {
		body += `<hr><p class="text-body-secondary small mb-0">Request ID: ` + html.EscapeString(requestID) + `</p>`
	}
	body += `</div></div></div></div></div>`
	return renderPage(title, body)
}

func errorTitleForStatus(code int) string {
	switch code {
	case fiber.StatusBadRequest:
		return "Bad Request"
	case fiber.StatusUnauthorized:
		return "Unauthorized"
	case fiber.StatusForbidden:
		return "Forbidden"
	case fiber.StatusNotFound:
		return "Not Found"
	case fiber.StatusTooManyRequests:
		return "Too Many Requests"
	default:
		return fmt.Sprintf("Error %d", code)
	}
}

func errorMessageForScope(code int, scope string) string {
	switch scope {
	case "admin":
		switch code {
		case fiber.StatusNotFound:
			return "The requested admin page was not found."
		case fiber.StatusForbidden:
			return "You do not have permission to view this page."
		default:
			return "An admin request failed."
		}
	case "client":
		switch code {
		case fiber.StatusNotFound:
			return "The requested client page was not found."
		case fiber.StatusForbidden:
			return "This client action is not allowed."
		default:
			return "A client request failed."
		}
	default:
		switch code {
		case fiber.StatusNotFound:
			return "The requested page was not found."
		case fiber.StatusTooManyRequests:
			return "Too many requests. Please try again later."
		default:
			return "Something went wrong."
		}
	}
}
