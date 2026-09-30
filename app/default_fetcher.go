package app

import (
	"context"
	"errors"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
)

// errNoDefaultOLT is returned when the registry has no default OLT to serve a
// trap or power-monitor lookup.
var errNoDefaultOLT = errors.New("no default OLT registered")

// defaultOLTFetcher implements the trap package's ONUDetailFetcher and
// ONUListFetcher by resolving the registry's default OLT on every call. The
// trap handler, batcher and power monitor live for the whole process, so
// capturing the default usecase once at startup would leave them bound to a
// closed SNMP pool after Reconcile rebuilds the default OLT.
type defaultOLTFetcher struct {
	reg *OLTRegistry
}

func (f defaultOLTFetcher) GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
	e, ok := f.reg.GetDefault()
	if !ok {
		return nil, errNoDefaultOLT
	}
	return e.UC.GetByBoardIDAndPonID(ctx, boardID, ponID)
}

func (f defaultOLTFetcher) GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
	e, ok := f.reg.GetDefault()
	if !ok {
		return model.ONUCustomerInfo{}, errNoDefaultOLT
	}
	return e.UC.GetByBoardIDPonIDAndOnuID(ctx, boardID, ponID, onuID)
}

func (f defaultOLTFetcher) InvalidateONUCache(ctx context.Context, boardID, ponID, onuID int) error {
	e, ok := f.reg.GetDefault()
	if !ok {
		return errNoDefaultOLT
	}
	return e.UC.InvalidateONUCache(ctx, boardID, ponID, onuID)
}
