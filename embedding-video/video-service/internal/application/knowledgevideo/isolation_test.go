package knowledgevideo

import (
	"context"
	"testing"
)

func TestIsolationImportDoesNotDependOnOrdinaryVideoOrVectorChain(t *testing.T) {
	deps := newImportTestDependencies(t)
	if _, err := deps.service.Import(context.Background(), deps.validInput(t)); err != nil {
		t.Fatal(err)
	}
}
