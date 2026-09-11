package xlsxsecurity

import "github.com/xuri/excelize/v2"

// ExcelizeDependencyAnchor holds a compile-time reference to Excelize until
// workbook import/export is implemented in a follow-up commit.
var ExcelizeDependencyAnchor = excelize.NewFile
