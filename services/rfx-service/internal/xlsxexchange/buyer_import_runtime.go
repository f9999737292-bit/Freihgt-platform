package xlsxexchange

import (
	"context"

	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

const (
	contextCheckRowInterval  = 128
	contextCheckCellInterval = 256
)

type openWorkbookFn func(data []byte) (workbookReader, error)

// internalParseConfig holds package-internal test hooks. Production callers use fixed limits.
type internalParseConfig struct {
	contentType  string
	openWorkbook openWorkbookFn
}

func productionParseConfig() internalParseConfig {
	return internalParseConfig{
		contentType:  "",
		openWorkbook: defaultOpenWorkbook,
	}
}

func productionSecurityLimits() xlsxsecurity.Limits {
	return xlsxsecurity.DefaultLimits()
}

func productionImportLimits() BuyerImportLimits {
	return DefaultBuyerImportLimits()
}

type buyerImportParser struct {
	ctx            context.Context
	issues         *issueCollector
	securityLimits xlsxsecurity.Limits
	importLimits   BuyerImportLimits
	contentType    string
	openWorkbook   openWorkbookFn
	rowChecks      uint64
	cellChecks     uint64
}

func newBuyerImportParser(ctx context.Context, cfg internalParseConfig) *buyerImportParser {
	openFn := cfg.openWorkbook
	if openFn == nil {
		openFn = defaultOpenWorkbook
	}
	return &buyerImportParser{
		ctx:            ctx,
		issues:         newIssueCollector(),
		securityLimits: productionSecurityLimits(),
		importLimits:   productionImportLimits(),
		contentType:    cfg.contentType,
		openWorkbook:   openFn,
	}
}

func (p *buyerImportParser) checkContext() error {
	return p.ctx.Err()
}

func (p *buyerImportParser) checkContextEveryRow() error {
	p.rowChecks++
	if p.rowChecks%contextCheckRowInterval == 0 {
		return p.ctx.Err()
	}
	return nil
}

func (p *buyerImportParser) checkContextEveryCell() error {
	p.cellChecks++
	if p.cellChecks%contextCheckCellInterval == 0 {
		return p.ctx.Err()
	}
	return nil
}

func (p *buyerImportParser) shouldStop() bool {
	return p.issues.stopped()
}
