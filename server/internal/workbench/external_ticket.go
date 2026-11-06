package workbench

import (
	"errors"
	"regexp"
)

var externalTicketPattern = regexp.MustCompile(`^[A-Za-z0-9-]{0,32}$`)

func ValidateExternalTicket(value string) error {
	if !externalTicketPattern.MatchString(value) {
		return errors.New("external ticket number must contain at most 32 letters, digits or hyphens")
	}
	return nil
}
