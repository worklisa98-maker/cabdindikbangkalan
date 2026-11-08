package services

import (
	"fmt"
	"strings"
)

func activeClass12StudentCondition(alias string) string {
	trimmedAlias := strings.TrimSpace(alias)
	if trimmedAlias == "" {
		trimmedAlias = "c"
	}
	return fmt.Sprintf("COALESCE(%s.import_batch_id, 0) <> 0", trimmedAlias)
}
