//nolint:revive // exported
package expression

import (
	"fmt"

	"github.com/go-faker/faker/v4"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// =============================================================================
// Built-in AI Expression Function (method on UnifiedEnv)
// =============================================================================
//
// The ai() function resolves a variable with metadata hints for AI.
// It behaves like {{ varName }} but includes description and type metadata.
//
// Usage: ai("varName", "description", "type")
// Returns: value if exists, error if not found

// helperUUID generates a new UUID string. Defaults to v4.
// Usage in expressions: uuid() or uuid("v4") or uuid("v7")
func helperUUID(args ...string) (string, error) {
	version := "v4"
	if len(args) > 0 {
		version = args[0]
	}

	switch version {
	case "v4":
		return uuid.New().String(), nil
	case "v7":
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("uuid: failed to generate v7: %w", err)
		}
		return id.String(), nil
	default:
		return "", fmt.Errorf("uuid: unsupported version %q, use \"v4\" or \"v7\"", version)
	}
}

// helperULID generates a new ULID string.
// Usage in expressions: ulid()
func helperULID() string {
	return ulid.Make().String()
}

// fakerNamespaceMap is the shared faker map built once at package init. It's
// stateless (every value is a plain wrapper calling the underlying go-faker
// function), so reusing a single map across all expression evaluations avoids
// allocating 34 closures + a new map on every call to buildExprEnv.
var fakerNamespaceMap = buildFakerNamespace()

// buildFakerNamespace returns a map of fake-data generators exposed to
// expressions under the "faker" root identifier.
//
// Usage in expressions: faker.email(), faker.name(), faker.url(), etc.
//
// The map values intentionally use fixed signatures (func() string / func() int64)
// so expr-lang can call them directly — go-faker's native variadic `opts` params
// are not forwarded, we always use defaults.
func buildFakerNamespace() map[string]any {
	return map[string]any{
		// Personal
		"name":        func() string { return faker.Name() },
		"firstName":   func() string { return faker.FirstName() },
		"lastName":    func() string { return faker.LastName() },
		"titleMale":   func() string { return faker.TitleMale() },
		"titleFemale": func() string { return faker.TitleFemale() },

		// Contact
		"email":       func() string { return faker.Email() },
		"phoneNumber": func() string { return faker.Phonenumber() },

		// Internet
		"url":        func() string { return faker.URL() },
		"domainName": func() string { return faker.DomainName() },
		"ipv4":       func() string { return faker.IPv4() },
		"ipv6":       func() string { return faker.IPv6() },
		"macAddress": func() string { return faker.MacAddress() },
		"username":   func() string { return faker.Username() },
		"password":   func() string { return faker.Password() },

		// Text
		"word":      func() string { return faker.Word() },
		"sentence":  func() string { return faker.Sentence() },
		"paragraph": func() string { return faker.Paragraph() },

		// Date / time
		"date":       func() string { return faker.Date() },
		"time":       func() string { return faker.TimeString() },
		"monthName":  func() string { return faker.MonthName() },
		"dayOfWeek":  func() string { return faker.DayOfWeek() },
		"dayOfMonth": func() string { return faker.DayOfMonth() },
		"year":       func() string { return faker.YearString() },
		"century":    func() string { return faker.Century() },
		"timestamp":  func() string { return faker.Timestamp() },
		"timezone":   func() string { return faker.Timezone() },
		"unixTime":   faker.RandomUnixTime,

		// Payment
		"ccNumber":           func() string { return faker.CCNumber() },
		"ccType":             func() string { return faker.CCType() },
		"currency":           func() string { return faker.Currency() },
		"amountWithCurrency": func() string { return faker.AmountWithCurrency() },

		// IDs
		"uuid":      func() string { return faker.UUIDHyphenated() },
		"uuidDigit": func() string { return faker.UUIDDigit() },

		// Random int — go-faker returns a slice, wrap to return a single int.
		// faker.randomInt(max)      -> int in [0, max]
		// faker.randomInt(min, max) -> int in [min, max]
		"randomInt": func(args ...int) (int, error) {
			var ns []int
			var err error
			switch len(args) {
			case 1:
				ns, err = faker.RandomInt(0, args[0], 1)
			case 2:
				ns, err = faker.RandomInt(args[0], args[1], 1)
			default:
				return 0, fmt.Errorf("faker.randomInt: expected 1 or 2 arguments, got %d", len(args))
			}
			if err != nil {
				return 0, err
			}
			if len(ns) == 0 {
				return 0, fmt.Errorf("faker.randomInt: no values generated")
			}
			return ns[0], nil
		},
	}
}

// helperAI returns the value of varName if it exists, otherwise returns an error.
// The description and varType parameters are metadata hints for AI tooling.
func (e *UnifiedEnv) helperAI(name, description, varType string) (any, error) {
	if name == "" {
		return nil, fmt.Errorf("ai: variable name is required")
	}

	if value, ok := e.Get(name); ok {
		return value, nil
	}

	return nil, fmt.Errorf("ai: variable %q not found", name)
}
